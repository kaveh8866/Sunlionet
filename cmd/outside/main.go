//go:build outside
// +build outside

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"filippo.io/age"
	"github.com/kaveh/shadownet-agent/pkg/bundle"
	"github.com/kaveh/shadownet-agent/pkg/outsidectl"
	"github.com/kaveh/shadownet-agent/pkg/profile"
	"github.com/kaveh/shadownet-agent/pkg/relay"
)

type bundleWrapper struct {
	Header     bundle.BundleHeader `json:"header"`
	Ciphertext string              `json:"ciphertext"`
}

type distManifest struct {
	Header             bundle.BundleHeader          `json:"header"`
	IssuerPublicB64URL string                       `json:"issuer_public_b64url"`
	BundleSHA256B64URL string                       `json:"bundle_sha256_b64url"`
	BundleSizeBytes    int                          `json:"bundle_size_bytes"`
	URIChars           int                          `json:"uri_chars"`
	QRFile             string                       `json:"qr_file,omitempty"`
	ChunksFile         string                       `json:"chunks_file,omitempty"`
	ChunksJSONFile     string                       `json:"chunks_json_file,omitempty"`
	Selection          outsidectl.SelectionManifest `json:"selection"`
}

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--verify" {
		verifyArgs := os.Args[2:]
		if len(verifyArgs) > 0 && !strings.HasPrefix(verifyArgs[0], "-") {
			verifyArgs = append([]string{"--bundle", verifyArgs[0]}, verifyArgs[1:]...)
		}
		if err := runVerify(verifyArgs); err != nil {
			log.Fatalf("verify: %v", err)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "relay" {
		if err := runRelay(os.Args[2:]); err != nil {
			log.Fatalf("relay: %v", err)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "verify" {
		if err := runVerify(os.Args[2:]); err != nil {
			log.Fatalf("verify: %v", err)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "keygen" {
		if err := runKeygen(os.Args[2:]); err != nil {
			log.Fatalf("keygen: %v", err)
		}
		return
	}
	if err := runGenerate(os.Args[1:]); err != nil {
		log.Fatalf("generate: %v", err)
	}
}

func runRelay(args []string) error {
	fs := flag.NewFlagSet("relay", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		listen               = fs.String("listen", "0.0.0.0:8081", "Listen address (host:port)")
		dataDir              = fs.String("data-dir", "./relay-data", "Directory for relay storage (envelopes are end-to-end encrypted)")
		allowNonLocal        = fs.Bool("allow-nonlocal", true, "Allow binding to non-localhost addresses")
		minPoWBits           = fs.Int("min-pow-bits", 0, "Minimum proof-of-work bits required for push requests (0 disables)")
		ipRatePerMin         = fs.Int("ip-rate-per-min", 1200, "Max requests per minute per source IP")
		mailboxRatePerMin    = fs.Int("mailbox-rate-per-min", 240, "Max requests per minute per mailbox")
		maxPendingPerMailbox = fs.Int("max-pending-per-mailbox", 10000, "Maximum queued messages per mailbox")
		maxTotalPending      = fs.Int("max-total-pending", 250000, "Maximum queued messages across all mailboxes")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	r, err := relay.NewFileRelay(*dataDir, relay.FileRelayOptions{
		MaxPendingPerMailbox: *maxPendingPerMailbox,
		MaxTotalPending:      *maxTotalPending,
	})
	if err != nil {
		return err
	}
	srv, err := relay.NewServer(*listen, r, relay.ServerOptions{
		AllowNonLocal:          *allowNonLocal,
		MinPoWBits:             *minPoWBits,
		IPRateLimitPerMin:      *ipRatePerMin,
		MailboxRateLimitPerMin: *mailboxRatePerMin,
	})
	if err != nil {
		return err
	}

	if err := srv.Start(); err != nil {
		return err
	}
	log.Printf("relay listening on %s data_dir=%s", *listen, *dataDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigs:
	case <-ctx.Done():
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	return nil
}

func runGenerate(args []string) error {
	fs := flag.NewFlagSet("outside", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		profilesPath     = fs.String("profiles", "", "Path to profiles JSON (single profile object or array)")
		profilesDir      = fs.String("profiles-dir", "", "Directory containing profile JSON files")
		outDir           = fs.String("out", "./dist", "Output directory")
		templatesDir     = fs.String("templates-dir", "./templates", "Directory containing sing-box outbound templates (*.json)")
		signingKeyPath   = fs.String("signing-key", "", "Path to Ed25519 private key (base64url seed(32) or private key(64))")
		recipientPubPath = fs.String("recipient-pub", "", "Optional path to age X25519 recipient public key (one line)")
		allowPlaintext   = fs.Bool("allow-plaintext", false, "Allow signed-but-unencrypted bundles when --recipient-pub is not provided")
		minAgentVersion  = fs.String("min-agent-version", "1.0.0", "Min agent version field in payload")
		bundleTTLSec     = fs.Int64("bundle-ttl-sec", 7*24*3600, "Bundle expires_at = created_at + ttl (seconds)")
		maxProfiles      = fs.Int("max-profiles", 20, "Maximum profiles per bundle")
		maxPerFamily     = fs.Int("max-per-family", 10, "Maximum profiles per family in a bundle")
		writeSummary     = fs.Bool("write-summary", true, "Write dist/summary.txt")
		qrThresholdChars = fs.Int("qr-threshold-chars", 1200, "Write dist/qr_payload.txt only if bundle.uri length is <= this threshold")
		chunkThreshold   = fs.Int("chunk-threshold-chars", 1800, "Prepare dist/bundle.chunks.txt if bundle.uri length is > this threshold")
		chunkSize        = fs.Int("chunk-size-chars", 900, "Max characters per chunk in dist/bundle.chunks.txt")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*signingKeyPath) == "" {
		return fmt.Errorf("missing --signing-key")
	}

	priv, pub, signerKeyID, err := outsidectl.LoadEd25519PrivateKeyFile(*signingKeyPath)
	if err != nil {
		return fmt.Errorf("load signing key: %w", err)
	}
	issuerPubB64 := base64.RawURLEncoding.EncodeToString(pub)

	var recipientPub string
	if strings.TrimSpace(*recipientPubPath) != "" {
		r, _, err := outsidectl.LoadAgeRecipientFile(*recipientPubPath)
		if err != nil {
			return fmt.Errorf("load recipient pub: %w", err)
		}
		recipientPub = r
	}

	now := time.Now().Unix()

	rawCandidates, err := outsidectl.LoadCandidates(*profilesPath, *profilesDir)
	if err != nil {
		return err
	}

	scored, excluded := outsidectl.ValidateNormalizeAndScore(rawCandidates, now)
	selection := outsidectl.SelectForBundle(scored, outsidectl.SelectionParams{
		MaxProfiles:  *maxProfiles,
		MaxPerFamily: *maxPerFamily,
		NowUnix:      now,
	})
	selection.Manifest.Excluded = append(selection.Manifest.Excluded, excluded...)
	sort.SliceStable(selection.Manifest.Excluded, func(i, j int) bool { return selection.Manifest.Excluded[i].ID < selection.Manifest.Excluded[j].ID })

	includedProfiles := make([]profile.Profile, 0, len(selection.Included))
	for _, c := range selection.Included {
		p := c.Profile
		p.Source.PublisherKey = signerKeyID
		includedProfiles = append(includedProfiles, p)
	}

	templates := map[string]bundle.Template{}
	for _, p := range includedProfiles {
		name := templateFileName(p)
		full := filepath.Join(*templatesDir, filepath.Clean(name))
		b, err := os.ReadFile(full)
		if err != nil {
			return fmt.Errorf("missing outbound template for profile %q (looked for %s): %w", p.ID, full, err)
		}
		key := templateKey(p)
		if _, ok := templates[key]; !ok {
			templates[key] = bundle.Template{TemplateText: string(b)}
		}
	}

	payload := bundle.BundlePayload{
		SchemaVersion:   1,
		MinAgentVersion: *minAgentVersion,
		Profiles:        includedProfiles,
		Revocations:     nil,
		PolicyOverrides: bundle.PolicyOverrides{},
		Templates:       templates,
		Notes: map[string]string{
			"issuer_key_id": signerKeyID,
			"created_at":    fmt.Sprintf("%d", now),
		},
	}

	bundleID := fmt.Sprintf("bndl_%d_%s", now, strings.TrimPrefix(signerKeyID, "ed25519:"))
	opts := bundle.GenerateOptions{
		RecipientPublicKey: recipientPub,
		AllowPlaintext:     *allowPlaintext,
		SignerKeyID:        signerKeyID,
		BundleID:           bundleID,
		Seq:                1,
		CreatedAt:          now,
		ExpiresAt:          now + *bundleTTLSec,
	}

	bundleBytes, err := bundle.GenerateBundleWithOptions(&payload, priv, opts)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*outDir, 0o700); err != nil {
		return err
	}

	var wrapper bundleWrapper
	if err := json.Unmarshal(bundleBytes, &wrapper); err != nil {
		return fmt.Errorf("internal: generated invalid wrapper json: %w", err)
	}

	manifest := distManifest{
		Header:             wrapper.Header,
		IssuerPublicB64URL: issuerPubB64,
		Selection:          selection.Manifest,
	}

	if err := writeFileAtomic(filepath.Join(*outDir, "bundle.snb.json"), bundleBytes, 0o600); err != nil {
		return err
	}
	uri := "snb://v2:" + base64.RawURLEncoding.EncodeToString(bundleBytes)
	if err := writeFileAtomic(filepath.Join(*outDir, "bundle.uri.txt"), []byte(uri+"\n"), 0o600); err != nil {
		return err
	}

	sum := sha256.Sum256(bundleBytes)
	manifest.BundleSHA256B64URL = base64.RawURLEncoding.EncodeToString(sum[:])
	manifest.BundleSizeBytes = len(bundleBytes)
	manifest.URIChars = len(uri)

	if *qrThresholdChars > 0 && len(uri) <= *qrThresholdChars {
		qrPath := filepath.Join(*outDir, "qr_payload.txt")
		if err := writeFileAtomic(qrPath, []byte(uri+"\n"), 0o600); err != nil {
			return err
		}
		manifest.QRFile = "qr_payload.txt"
	}

	if *chunkThreshold > 0 && len(uri) > *chunkThreshold {
		chunks, err := outsidectl.ChunkString(uri, *chunkSize)
		if err != nil {
			return err
		}
		lines, err := outsidectl.RenderChunkLines(chunks)
		if err != nil {
			return err
		}
		chunksPath := filepath.Join(*outDir, "bundle.chunks.txt")
		if err := writeFileAtomic(chunksPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			return err
		}
		chunksJSONPath := filepath.Join(*outDir, "bundle.chunks.json")
		if err := writeJSONAtomic(chunksJSONPath, chunks, 0o600); err != nil {
			return err
		}
		manifest.ChunksFile = "bundle.chunks.txt"
		manifest.ChunksJSONFile = "bundle.chunks.json"
	}

	if err := writeJSONAtomic(filepath.Join(*outDir, "manifest.json"), manifest, 0o600); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(*outDir, "bundle.sig.b64url"), []byte(wrapper.Header.Signature+"\n"), 0o600); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(*outDir, "issuer_pub.b64url"), []byte(issuerPubB64+"\n"), 0o600); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(*outDir, "issuer_key_id.txt"), []byte(signerKeyID+"\n"), 0o600); err != nil {
		return err
	}
	if *writeSummary {
		summary := renderSummary(manifest.Selection, wrapper.Header, manifest.BundleSHA256B64URL, manifest.URIChars, manifest.QRFile != "", manifest.ChunksFile != "")
		if err := writeFileAtomic(filepath.Join(*outDir, "summary.txt"), []byte(summary), 0o600); err != nil {
			return err
		}
	}

	log.Printf("bundle=%s profiles=%d excluded=%d cipher=%s issuer=%s", filepath.Join(*outDir, "bundle.snb.json"), len(includedProfiles), len(manifest.Selection.Excluded), wrapper.Header.Cipher, signerKeyID)
	return nil
}

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		bundlePath      = fs.String("bundle", "", "Path to bundle.snb.json")
		uri             = fs.String("uri", "", "Bundle URI (snb://v2:...)")
		uriFile         = fs.String("uri-file", "", "Path to a file containing a bundle URI (snb://v2:...)")
		signerPubPath   = fs.String("signer-pub", "", "Path to trusted signer Ed25519 public key (base64url)")
		ageIdentityPath = fs.String("age-identity", "", "Optional path to age identity (required to decrypt age-x25519 bundles)")
		requireDecrypt  = fs.Bool("require-decrypt", false, "Fail if bundle is encrypted and payload cannot be decrypted/validated")
		jsonOut         = fs.Bool("json", false, "Write machine-readable verification output to stdout")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*bundlePath) == "" && strings.TrimSpace(*uri) == "" && strings.TrimSpace(*uriFile) == "" {
		return fmt.Errorf("missing --bundle or --uri or --uri-file")
	}
	if strings.TrimSpace(*signerPubPath) == "" {
		return fmt.Errorf("missing --signer-pub")
	}

	pub, _, err := outsidectl.LoadEd25519PublicKeyFile(*signerPubPath)
	if err != nil {
		return fmt.Errorf("load signer pub: %w", err)
	}

	var raw []byte
	switch {
	case strings.TrimSpace(*bundlePath) != "":
		b, err := os.ReadFile(*bundlePath)
		if err != nil {
			return err
		}
		raw = b
	case strings.TrimSpace(*uriFile) != "":
		b, err := os.ReadFile(*uriFile)
		if err != nil {
			return err
		}
		u := strings.TrimSpace(string(b))
		if u == "" {
			return fmt.Errorf("empty uri file")
		}
		bin, err := outsidectl.DecodeBundleURI(u)
		if err != nil {
			return err
		}
		raw = bin
	default:
		bin, err := outsidectl.DecodeBundleURI(strings.TrimSpace(*uri))
		if err != nil {
			return err
		}
		raw = bin
	}

	var ageIdentity *age.X25519Identity
	if strings.TrimSpace(*ageIdentityPath) != "" {
		id, _, err := outsidectl.LoadAgeIdentityFile(*ageIdentityPath)
		if err != nil {
			return fmt.Errorf("load age identity: %w", err)
		}
		ageIdentity = id
	}

	res, err := bundle.VerifyBundle(raw, pub, bundle.VerifyOptions{
		AgeIdentity:    ageIdentity,
		RequireDecrypt: *requireDecrypt,
	})
	if err != nil {
		if *jsonOut {
			out := map[string]any{
				"ok":    false,
				"error": err.Error(),
			}
			if res != nil {
				out["header"] = res.Header
			}
			if verr, ok := err.(*bundle.VerifyError); ok {
				out["issues"] = verr.Issues
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Fprintln(os.Stdout, string(b))
		}
		return err
	}

	if *jsonOut {
		out := map[string]any{
			"ok":     true,
			"header": res.Header,
		}
		if res.Payload != nil {
			out["payload_profiles"] = len(res.Payload.Profiles)
			out["schema_version"] = res.Payload.SchemaVersion
			out["min_agent_version"] = res.Payload.MinAgentVersion
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Fprintln(os.Stdout, string(b))
		return nil
	}

	log.Printf("ok verify bundle_id=%s issuer=%s cipher=%s created_at=%d expires_at=%d sha256=%s", res.Header.BundleID, res.Header.PublisherKeyID, res.Header.Cipher, res.Header.CreatedAt, res.Header.ExpiresAt, res.BundleSHA256B64URL)
	if res.Payload == nil {
		log.Printf("note: header verified; payload not decrypted (provide --age-identity, or set --require-decrypt for CI)")
		return nil
	}

	familyCounts := map[profile.Family]int{}
	for _, p := range res.Payload.Profiles {
		familyCounts[p.Family]++
	}
	var fams []string
	for f := range familyCounts {
		fams = append(fams, string(f))
	}
	sort.Strings(fams)
	var parts []string
	for _, f := range fams {
		parts = append(parts, fmt.Sprintf("%s=%d", f, familyCounts[profile.Family(f)]))
	}
	log.Printf("payload profiles=%d families=%s", len(res.Payload.Profiles), strings.Join(parts, ","))
	return nil
}

func runKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		ed25519PrivPath = fs.String("ed25519-priv", "", "Path to write Ed25519 private key seed (base64url, 32 bytes)")
		ed25519PubPath  = fs.String("ed25519-pub", "", "Path to write Ed25519 public key (base64url, 32 bytes)")
		ageIDPath       = fs.String("age-identity", "", "Optional path to write age X25519 identity (AGE-SECRET-KEY-1...)")
		ageRecipient    = fs.String("age-recipient", "", "Optional path to write age X25519 recipient (age1...)")
		force           = fs.Bool("force", false, "Overwrite existing files")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*ed25519PrivPath) == "" && strings.TrimSpace(*ageIDPath) == "" {
		return fmt.Errorf("nothing to generate: set --ed25519-priv or --age-identity")
	}

	if strings.TrimSpace(*ed25519PrivPath) != "" || strings.TrimSpace(*ed25519PubPath) != "" {
		if strings.TrimSpace(*ed25519PrivPath) == "" || strings.TrimSpace(*ed25519PubPath) == "" {
			return fmt.Errorf("ed25519 requires both --ed25519-priv and --ed25519-pub")
		}
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		seedB64 := base64.RawURLEncoding.EncodeToString(priv.Seed())
		pubB64 := base64.RawURLEncoding.EncodeToString(pub)

		if err := writeNewFileAtomic(*ed25519PrivPath, []byte(seedB64+"\n"), 0o600, *force); err != nil {
			return err
		}
		if err := writeNewFileAtomic(*ed25519PubPath, []byte(pubB64+"\n"), 0o644, *force); err != nil {
			return err
		}
		log.Printf("ed25519 key_id=%s", "ed25519:"+outsidectl.Fingerprint16(pub))
	}

	if strings.TrimSpace(*ageIDPath) != "" {
		id, err := age.GenerateX25519Identity()
		if err != nil {
			return err
		}
		if strings.TrimSpace(*ageRecipient) == "" {
			return fmt.Errorf("age requires --age-recipient when --age-identity is set")
		}
		if err := writeNewFileAtomic(*ageIDPath, []byte(id.String()+"\n"), 0o600, *force); err != nil {
			return err
		}
		if err := writeNewFileAtomic(*ageRecipient, []byte(id.Recipient().String()+"\n"), 0o644, *force); err != nil {
			return err
		}
		log.Printf("age recipient_key_id=%s", "age-x25519:"+outsidectl.Fingerprint16String(id.Recipient().String()))
	}

	return nil
}

func writeNewFileAtomic(path string, data []byte, perm os.FileMode, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing file: %s (use --force)", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(path, data, perm)
}

func templateKey(p profile.Profile) string {
	if strings.TrimSpace(p.TemplateRef) != "" {
		return p.TemplateRef
	}
	return string(p.Family)
}

func templateFileName(p profile.Profile) string {
	if strings.TrimSpace(p.TemplateRef) != "" {
		name := p.TemplateRef
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			name += ".json"
		}
		return name
	}
	return string(p.Family) + ".json"
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeJSONAtomic(path string, v any, perm os.FileMode) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return writeFileAtomic(path, b, perm)
}

func renderSummary(sel outsidectl.SelectionManifest, header bundle.BundleHeader, bundleSHA256B64URL string, uriChars int, hasQR bool, hasChunks bool) string {
	var b strings.Builder
	b.WriteString("SunLionet Outside bundle\n")
	b.WriteString("bundle_id=" + header.BundleID + "\n")
	b.WriteString("issuer=" + header.PublisherKeyID + "\n")
	b.WriteString("cipher=" + header.Cipher + "\n")
	b.WriteString(fmt.Sprintf("created_at=%d expires_at=%d\n", header.CreatedAt, header.ExpiresAt))
	if bundleSHA256B64URL != "" {
		b.WriteString("bundle_sha256_b64url=" + bundleSHA256B64URL + "\n")
	}
	if uriChars > 0 {
		b.WriteString(fmt.Sprintf("uri_chars=%d\n", uriChars))
	}
	if hasQR {
		b.WriteString("qr_payload=dist/qr_payload.txt\n")
	}
	if hasChunks {
		b.WriteString("chunks=dist/bundle.chunks.txt (SNBCHUNK/1)\n")
	}
	b.WriteString("\nIncluded:\n")
	for _, in := range sel.Included {
		b.WriteString(fmt.Sprintf("- %s (%s) score=%.2f\n", in.ID, in.Family, in.Score))
	}
	b.WriteString("\nExcluded:\n")
	if len(sel.Excluded) == 0 {
		b.WriteString("- (none)\n")
	} else {
		sort.SliceStable(sel.Excluded, func(i, j int) bool { return sel.Excluded[i].ID < sel.Excluded[j].ID })
		for _, ex := range sel.Excluded {
			id := strings.TrimSpace(ex.ID)
			if id == "" {
				id = "(unknown)"
			}
			b.WriteString(fmt.Sprintf("- %s: %s\n", id, ex.Reason))
		}
	}
	return b.String()
}
