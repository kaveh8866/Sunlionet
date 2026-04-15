package installer

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

type Checksums struct {
	SHA256 string
	SHA512 string
}

func VerifyFile(path string, expected Checksums) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	got, err := HashReader(f)
	if err != nil {
		return err
	}

	if expected.SHA256 != "" && !equalHex(expected.SHA256, got.SHA256) {
		return fmt.Errorf("checksum mismatch (sha256): expected=%s got=%s", expected.SHA256, got.SHA256)
	}
	if expected.SHA512 != "" && !equalHex(expected.SHA512, got.SHA512) {
		return fmt.Errorf("checksum mismatch (sha512): expected=%s got=%s", expected.SHA512, got.SHA512)
	}

	return nil
}

func HashReader(r io.Reader) (Checksums, error) {
	if r == nil {
		return Checksums{}, errors.New("hash: nil reader")
	}

	h256 := sha256.New()
	h512 := sha512.New()
	mw := io.MultiWriter(h256, h512)
	if _, err := io.Copy(mw, r); err != nil {
		return Checksums{}, err
	}

	return Checksums{
		SHA256: hex.EncodeToString(h256.Sum(nil)),
		SHA512: hex.EncodeToString(h512.Sum(nil)),
	}, nil
}

func equalHex(a, b string) bool {
	return normalizeHex(a) == normalizeHex(b)
}

func normalizeHex(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'F' {
			c = c - 'A' + 'a'
		}
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			out = append(out, c)
		}
	}
	return string(out)
}
