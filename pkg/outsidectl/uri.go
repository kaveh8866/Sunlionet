package outsidectl

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func DecodeBundleURI(uri string) ([]byte, error) {
	s := strings.TrimSpace(uri)
	if !strings.HasPrefix(s, "snb://v2:") {
		return nil, fmt.Errorf("invalid scheme or version (expected snb://v2:)")
	}
	body := strings.TrimPrefix(s, "snb://v2:")
	b, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode bundle base64: %w", err)
	}
	return b, nil
}
