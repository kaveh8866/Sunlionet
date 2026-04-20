package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
)

func MarshalCanonicalEventUnsigned(e *Event) ([]byte, error) {
	if e == nil {
		return nil, errors.New("ledger: event is nil")
	}
	type field struct {
		name  string
		value any
	}
	fields := []field{
		{name: "v", value: e.SchemaVersion},
		{name: "created_at", value: e.CreatedAt},
		{name: "author", value: e.Author},
		{name: "author_key_b64url", value: e.AuthorKeyB64},
		{name: "seq", value: e.Seq},
		{name: "prev", value: e.Prev},
		{name: "parents", value: e.Parents},
		{name: "kind", value: e.Kind},
		{name: "payload_hash_b64url", value: e.PayloadHashB64},
		{name: "payload_ref", value: e.PayloadRef},
	}

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, f := range fields {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(f.name)
		if err != nil {
			return nil, err
		}
		vb, err := json.Marshal(f.value)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
