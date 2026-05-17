package bundle

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"
	"text/template"
	"time"

	"filippo.io/age"
	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

type VerifyIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type VerifyError struct {
	Issues []VerifyIssue
}

func (e *VerifyError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "verification failed"
	}
	if len(e.Issues) == 1 {
		return e.Issues[0].Code + ": " + e.Issues[0].Message
	}
	var b strings.Builder
	b.WriteString("verification failed:")
	for _, it := range e.Issues {
		b.WriteString(" ")
		b.WriteString(it.Code)
		b.WriteString(": ")
		b.WriteString(it.Message)
		b.WriteString(";")
	}
	return strings.TrimSuffix(b.String(), ";")
}

func (e *VerifyError) add(code, msg string) {
	e.Issues = append(e.Issues, VerifyIssue{Code: code, Message: msg})
}

type VerifyOptions struct {
	NowUnix        int64
	AgeIdentity    *age.X25519Identity
	RequireDecrypt bool
}

type VerifyResult struct {
	Header              BundleHeader   `json:"header"`
	SignerKeyIDExpected string         `json:"signer_key_id_expected"`
	BundleSHA256B64URL  string         `json:"bundle_sha256_b64url"`
	Payload             *BundlePayload `json:"payload,omitempty"`
}

func VerifyBundle(bundleBytes []byte, trustedSigner ed25519.PublicKey, opts VerifyOptions) (*VerifyResult, error) {
	now := opts.NowUnix
	if now == 0 {
		now = time.Now().Unix()
	}
	if len(bundleBytes) == 0 || len(bundleBytes) > MaxBundleBytes {
		return nil, &VerifyError{Issues: []VerifyIssue{{Code: "wrapper_size", Message: "bundle size is invalid"}}}
	}
	if len(trustedSigner) != ed25519.PublicKeySize {
		return nil, ErrInvalidSignature
	}

	sum := sha256.Sum256(bundleBytes)
	res := &VerifyResult{
		BundleSHA256B64URL:  base64.RawURLEncoding.EncodeToString(sum[:]),
		SignerKeyIDExpected: Ed25519KeyID(trustedSigner),
	}

	var wrapper struct {
		Header     BundleHeader `json:"header"`
		Ciphertext string       `json:"ciphertext"`
	}
	if err := decodeStrict(bundleBytes, &wrapper); err != nil {
		return nil, &VerifyError{Issues: []VerifyIssue{{Code: "wrapper_parse", Message: err.Error()}}}
	}
	res.Header = wrapper.Header

	verr := &VerifyError{}
	if wrapper.Header.Magic != "SNB1" {
		verr.add("header_magic", "invalid magic (expected SNB1)")
	}
	if strings.TrimSpace(wrapper.Header.BundleID) == "" {
		verr.add("header_bundle_id", "missing bundle_id")
	}
	if strings.TrimSpace(wrapper.Header.PublisherKeyID) == "" {
		verr.add("header_publisher_key_id", "missing publisher_key_id")
	} else if wrapper.Header.PublisherKeyID != res.SignerKeyIDExpected {
		verr.add("issuer_mismatch", fmt.Sprintf("publisher_key_id=%q does not match trusted signer key id=%q", wrapper.Header.PublisherKeyID, res.SignerKeyIDExpected))
	}
	if wrapper.Header.Seq == 0 {
		verr.add("header_seq", "seq must be > 0")
	}
	if _, err := decodeBundleNonce(wrapper.Header.Nonce); err != nil {
		verr.add("header_nonce", err.Error())
	}
	if wrapper.Header.CreatedAt <= 0 {
		verr.add("header_created_at", "created_at must be > 0")
	}
	if wrapper.Header.ExpiresAt <= 0 {
		verr.add("header_expires_at", "expires_at must be > 0")
	}
	if wrapper.Header.CreatedAt > 0 && wrapper.Header.ExpiresAt > 0 && wrapper.Header.ExpiresAt < wrapper.Header.CreatedAt {
		verr.add("header_time_order", "expires_at must be >= created_at")
	}
	if wrapper.Header.CreatedAt > now+MaxClockSkewSeconds {
		verr.add("header_future", "created_at is too far in the future (clock skew)")
	}
	if wrapper.Header.ExpiresAt > 0 && wrapper.Header.CreatedAt > 0 && wrapper.Header.ExpiresAt-wrapper.Header.CreatedAt > MaxBundleTTLSeconds {
		verr.add("header_ttl", "bundle ttl exceeds maximum")
	}
	if now > wrapper.Header.ExpiresAt {
		verr.add("expired", "bundle expired")
	}

	switch wrapper.Header.Cipher {
	case "age-x25519":
		if !strings.HasPrefix(wrapper.Header.RecipientKeyID, "age-x25519:") {
			verr.add("recipient_key_id", "age-x25519 bundles must have recipient_key_id starting with age-x25519:")
		}
	case "none":
		if wrapper.Header.RecipientKeyID != "none" {
			verr.add("recipient_key_id", `cipher="none" requires recipient_key_id="none"`)
		}
	default:
		verr.add("cipher", "unknown cipher")
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(wrapper.Ciphertext)
	if err != nil {
		verr.add("ciphertext_b64", "invalid ciphertext base64url")
	} else if len(ciphertext) == 0 || len(ciphertext) > MaxCiphertextBytes {
		verr.add("ciphertext_size", "ciphertext size is invalid")
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(wrapper.Header.Signature)
	if err != nil {
		verr.add("signature_b64", "invalid signature base64url")
	} else if len(sigBytes) != ed25519.SignatureSize {
		verr.add("signature_size", "invalid signature size")
	}

	if len(verr.Issues) > 0 {
		return res, verr
	}

	headerCopy := wrapper.Header
	headerCopy.Signature = ""
	headerBytes, err := json.Marshal(headerCopy)
	if err != nil {
		return res, &VerifyError{Issues: []VerifyIssue{{Code: "header_marshal", Message: err.Error()}}}
	}
	if !ed25519.Verify(trustedSigner, signatureInput(headerBytes, ciphertext), sigBytes) {
		return res, ErrInvalidSignature
	}

	if wrapper.Header.Cipher == "age-x25519" && opts.AgeIdentity == nil && opts.RequireDecrypt {
		return res, &VerifyError{Issues: []VerifyIssue{{Code: "decrypt_required", Message: "encrypted bundle requires age identity"}}}
	}
	if wrapper.Header.Cipher == "age-x25519" && opts.AgeIdentity == nil {
		return res, nil
	}

	payloadBytes, err := decryptPayloadBytes(wrapper.Header, ciphertext, opts.AgeIdentity)
	if err != nil {
		if errors.Is(err, ErrDecryptionFailed) {
			return res, err
		}
		return res, &VerifyError{Issues: []VerifyIssue{{Code: "decrypt", Message: err.Error()}}}
	}

	var payload BundlePayload
	if err := decodeStrict(payloadBytes, &payload); err != nil {
		return res, &VerifyError{Issues: []VerifyIssue{{Code: "payload_parse", Message: err.Error()}}}
	}

	canonical, err := MarshalCanonicalPayload(&payload)
	if err != nil {
		return res, &VerifyError{Issues: []VerifyIssue{{Code: "payload_canonicalize", Message: err.Error()}}}
	}
	if !bytes.Equal(canonical, payloadBytes) {
		return res, &VerifyError{Issues: []VerifyIssue{{Code: "payload_not_canonical", Message: "payload JSON is not canonical"}}}
	}

	if err := validatePayloadStrict(&payload, wrapper.Header, now); err != nil {
		return res, err
	}
	res.Payload = &payload
	return res, nil
}

func decryptPayloadBytes(header BundleHeader, ciphertext []byte, id *age.X25519Identity) ([]byte, error) {
	switch header.Cipher {
	case "none":
		return ciphertext, nil
	case "age-x25519":
		if id == nil {
			return nil, fmt.Errorf("%w: missing age identity", ErrDecryptionFailed)
		}
		expected := "age-x25519:" + fingerprint16(id.Recipient().String())
		if strings.TrimSpace(header.RecipientKeyID) != expected {
			return nil, fmt.Errorf("%w: recipient_key_id mismatch (expected %s)", ErrDecryptionFailed, expected)
		}
		decReader, err := age.Decrypt(bytes.NewReader(ciphertext), id)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
		}
		b, err := io.ReadAll(io.LimitReader(decReader, MaxPayloadBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read decrypted payload: %w", err)
		}
		if len(b) > MaxPayloadBytes {
			return nil, fmt.Errorf("decrypted payload too large")
		}
		return b, nil
	default:
		return nil, ErrUnknownCipher
	}
}

func validatePayloadStrict(payload *BundlePayload, header BundleHeader, now int64) error {
	verr := &VerifyError{}
	if payload.SchemaVersion != 1 {
		verr.add("schema_version", fmt.Sprintf("unsupported schema_version=%d", payload.SchemaVersion))
	}
	if strings.TrimSpace(payload.MinAgentVersion) == "" {
		verr.add("min_agent_version", "missing min_agent_version")
	}

	if strings.TrimSpace(payload.Notes["issuer_key_id"]) == "" {
		verr.add("issuer_note", "missing notes.issuer_key_id")
	} else if payload.Notes["issuer_key_id"] != header.PublisherKeyID {
		verr.add("issuer_note_mismatch", "notes.issuer_key_id must match header.publisher_key_id")
	}

	seenIDs := map[string]bool{}
	seenEndpoints := map[string]bool{}
	for i := range payload.Profiles {
		p := payload.Profiles[i]
		np, err := profile.NormalizeForWire(p, now)
		if err != nil {
			verr.add("profile_invalid", fmt.Sprintf("profiles[%d] id=%q: %v", i, p.ID, err))
			continue
		}
		if !reflect.DeepEqual(np, p) {
			verr.add("profile_not_normalized", fmt.Sprintf("profiles[%d] id=%q is not normalized", i, p.ID))
		}
		if strings.TrimSpace(p.Source.PublisherKey) == "" {
			verr.add("profile_issuer_missing", fmt.Sprintf("profiles[%d] id=%q missing source.publisher_key", i, p.ID))
		} else if p.Source.PublisherKey != header.PublisherKeyID {
			verr.add("profile_issuer_mismatch", fmt.Sprintf("profiles[%d] id=%q source.publisher_key mismatch", i, p.ID))
		}
		if seenIDs[p.ID] {
			verr.add("profile_duplicate_id", fmt.Sprintf("duplicate profile id=%q", p.ID))
		}
		seenIDs[p.ID] = true
		ek := fmt.Sprintf("%s|%s|%d", p.Family, p.Endpoint.Host, p.Endpoint.Port)
		if seenEndpoints[ek] {
			verr.add("profile_duplicate_endpoint", fmt.Sprintf("duplicate endpoint=%s", ek))
		}
		seenEndpoints[ek] = true
		if p.ExpiresAt != 0 && p.ExpiresAt <= now {
			verr.add("profile_expired", fmt.Sprintf("profiles[%d] id=%q expired", i, p.ID))
		}
		if p.ExpiresAt != 0 && p.ExpiresAt < p.CreatedAt {
			verr.add("profile_time_order", fmt.Sprintf("profiles[%d] id=%q expires_at < created_at", i, p.ID))
		}
	}

	sampleProfileForTemplate := map[string]profile.Profile{}
	for i := range payload.Profiles {
		p := payload.Profiles[i]
		key := p.TemplateRef
		if strings.TrimSpace(key) == "" {
			key = string(p.Family)
		}
		if _, ok := sampleProfileForTemplate[key]; !ok {
			sampleProfileForTemplate[key] = p
		}
	}

	for k, v := range payload.Templates {
		if strings.TrimSpace(k) == "" {
			verr.add("template_key", "empty template key")
			continue
		}
		if strings.Contains(k, `\`) {
			verr.add("template_key", fmt.Sprintf("invalid template key=%q", k))
			continue
		}
		clean := path.Clean(k)
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
			verr.add("template_key", fmt.Sprintf("invalid template key=%q", k))
			continue
		}
		if strings.TrimSpace(v.TemplateText) == "" {
			verr.add("template_text", fmt.Sprintf("empty template_text for key=%q", k))
			continue
		}
		if prof, ok := sampleProfileForTemplate[k]; ok {
			tmpl, err := template.New("outbound").Parse(v.TemplateText)
			if err != nil {
				verr.add("template_render", fmt.Sprintf("failed to parse template for key=%q: %v", k, err))
				continue
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, prof); err != nil {
				verr.add("template_render", fmt.Sprintf("failed to execute template for key=%q: %v", k, err))
				continue
			}
			out := bytes.TrimSpace(buf.Bytes())
			if !json.Valid(out) {
				verr.add("template_render", fmt.Sprintf("rendered template is not valid JSON for key=%q", k))
			}
		}
	}

	for i := range payload.Profiles {
		p := payload.Profiles[i]
		key := p.TemplateRef
		if strings.TrimSpace(key) == "" {
			key = string(p.Family)
		}
		if _, ok := payload.Templates[key]; !ok {
			verr.add("template_missing", fmt.Sprintf("missing template for profile id=%q key=%q", p.ID, key))
		}
	}

	if len(verr.Issues) > 0 {
		return verr
	}
	return nil
}

func decodeStrict(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	tok, err := dec.Token()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("trailing JSON content: %v", tok)
}

func Ed25519KeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return "ed25519:" + base64.RawURLEncoding.EncodeToString(sum[:])[:16]
}
