package bundle

import (
	"bytes"
	"encoding/json"
	"sort"
)

func MarshalCanonicalPayload(payload *BundlePayload) ([]byte, error) {
	type field struct {
		name  string
		value any
	}

	fields := []field{
		{name: "schema_version", value: payload.SchemaVersion},
		{name: "min_agent_version", value: payload.MinAgentVersion},
		{name: "profiles", value: payload.Profiles},
		{name: "revocations", value: payload.Revocations},
		{name: "policy_overrides", value: payload.PolicyOverrides},
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

	if buf.Len() > 1 {
		buf.WriteByte(',')
	}
	buf.WriteString(`"templates":`)
	if err := writeSortedObject(&buf, payload.Templates); err != nil {
		return nil, err
	}

	buf.WriteByte(',')
	buf.WriteString(`"notes":`)
	if err := writeSortedObject(&buf, payload.Notes); err != nil {
		return nil, err
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func writeSortedObject[T any](buf *bytes.Buffer, m map[string]T) error {
	if len(m) == 0 {
		buf.WriteString(`{}`)
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return err
		}
		vb, err := json.Marshal(m[k])
		if err != nil {
			return err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		buf.Write(vb)
	}
	buf.WriteByte('}')
	return nil
}
