package outsidectl

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Chunk struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Index   int    `json:"index"`
	Total   int    `json:"total"`
	Data    string `json:"data"`
}

func ChunkString(input string, maxChunkLen int) ([]Chunk, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return nil, fmt.Errorf("empty input")
	}
	if maxChunkLen <= 0 {
		return nil, fmt.Errorf("invalid chunk size")
	}

	sum := sha256.Sum256([]byte(s))
	id := base64.RawURLEncoding.EncodeToString(sum[:])[:12]

	var parts []string
	for i := 0; i < len(s); i += maxChunkLen {
		end := i + maxChunkLen
		if end > len(s) {
			end = len(s)
		}
		parts = append(parts, s[i:end])
	}
	total := len(parts)
	out := make([]Chunk, 0, total)
	for i := range parts {
		out = append(out, Chunk{
			Version: 1,
			ID:      id,
			Index:   i + 1,
			Total:   total,
			Data:    parts[i],
		})
	}
	return out, nil
}

func RenderChunkLines(chunks []Chunk) ([]string, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks")
	}
	id := chunks[0].ID
	total := chunks[0].Total
	for _, c := range chunks {
		if c.Version != 1 {
			return nil, fmt.Errorf("unsupported chunk version")
		}
		if c.ID != id {
			return nil, fmt.Errorf("mixed chunk ids")
		}
		if c.Total != total {
			return nil, fmt.Errorf("inconsistent total")
		}
		if c.Index <= 0 || c.Index > c.Total {
			return nil, fmt.Errorf("invalid index")
		}
		if strings.TrimSpace(c.Data) == "" {
			return nil, fmt.Errorf("empty chunk data")
		}
	}
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Index < chunks[j].Index })

	lines := make([]string, 0, len(chunks))
	for _, c := range chunks {
		lines = append(lines, fmt.Sprintf("SNBCHUNK/1 %s %d/%d %s", c.ID, c.Index, c.Total, c.Data))
	}
	return lines, nil
}

func ParseChunkLine(line string) (Chunk, error) {
	s := strings.TrimSpace(line)
	if s == "" {
		return Chunk{}, fmt.Errorf("empty line")
	}
	parts := strings.SplitN(s, " ", 4)
	if len(parts) != 4 {
		return Chunk{}, fmt.Errorf("invalid chunk format")
	}
	if parts[0] != "SNBCHUNK/1" {
		return Chunk{}, fmt.Errorf("unsupported chunk version")
	}
	id := strings.TrimSpace(parts[1])
	if id == "" {
		return Chunk{}, fmt.Errorf("missing chunk id")
	}
	idxParts := strings.SplitN(parts[2], "/", 2)
	if len(idxParts) != 2 {
		return Chunk{}, fmt.Errorf("invalid chunk index")
	}
	i, err := strconv.Atoi(idxParts[0])
	if err != nil {
		return Chunk{}, fmt.Errorf("invalid chunk index")
	}
	n, err := strconv.Atoi(idxParts[1])
	if err != nil {
		return Chunk{}, fmt.Errorf("invalid chunk total")
	}
	data := strings.TrimSpace(parts[3])
	if data == "" {
		return Chunk{}, fmt.Errorf("empty chunk data")
	}
	if i <= 0 || n <= 0 || i > n {
		return Chunk{}, fmt.Errorf("invalid chunk range")
	}
	return Chunk{Version: 1, ID: id, Index: i, Total: n, Data: data}, nil
}

func JoinChunkLines(lines []string) (string, error) {
	if len(lines) == 0 {
		return "", fmt.Errorf("no lines")
	}
	var chunks []Chunk
	for _, ln := range lines {
		c, err := ParseChunkLine(ln)
		if err != nil {
			return "", err
		}
		chunks = append(chunks, c)
	}
	return JoinChunks(chunks)
}

func JoinChunks(chunks []Chunk) (string, error) {
	if len(chunks) == 0 {
		return "", fmt.Errorf("no chunks")
	}
	id := chunks[0].ID
	total := chunks[0].Total
	seen := map[int]bool{}
	for _, c := range chunks {
		if c.Version != 1 {
			return "", fmt.Errorf("unsupported chunk version")
		}
		if c.ID != id {
			return "", fmt.Errorf("mixed chunk ids")
		}
		if c.Total != total {
			return "", fmt.Errorf("inconsistent total")
		}
		if c.Index <= 0 || c.Index > c.Total {
			return "", fmt.Errorf("invalid index")
		}
		if seen[c.Index] {
			return "", fmt.Errorf("duplicate chunk index")
		}
		seen[c.Index] = true
	}
	if len(seen) != total {
		return "", fmt.Errorf("missing chunks (%d/%d)", len(seen), total)
	}
	sort.Slice(chunks, func(i, j int) bool { return chunks[i].Index < chunks[j].Index })
	var b strings.Builder
	for _, c := range chunks {
		b.WriteString(c.Data)
	}
	return b.String(), nil
}
