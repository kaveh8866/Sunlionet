package bundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	ChunkMagic               = "SNCE"
	ChunkVersion        byte = 1
	ChunkHeaderSize          = 4 + 1 + 1 + 16 + 2 + 2 + 2 + 4 + 2 + 32 + 32
	MaxChunkDataSize         = 64 * 1024
	MaxShardCount            = 255
	DefaultMaxCacheByte      = 2 << 20
)

var (
	ErrChunkInvalid       = errors.New("chunk engine: invalid chunk")
	ErrChunkChecksum      = errors.New("chunk engine: checksum mismatch")
	ErrChunkCacheLimit    = errors.New("chunk engine: cache limit exceeded")
	ErrChunkUnrecoverable = errors.New("chunk engine: unrecoverable")
)

type EncodedChunk struct {
	BundleID      [16]byte
	Index         int
	DataShards    int
	ParityShards  int
	PayloadSize   int
	ShardSize     int
	PayloadSHA256 [32]byte
	ChunkSHA256   [32]byte
	Data          []byte
}

type ChunkOptions struct {
	DataShards   int
	ParityShards int
	ShardSize    int
}

func EncodeErasureChunks(payload []byte, opt ChunkOptions) ([]EncodedChunk, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("%w: empty payload", ErrChunkInvalid)
	}
	if len(payload) > MaxBundleBytes {
		return nil, fmt.Errorf("%w: payload too large", ErrChunkInvalid)
	}
	if opt.ShardSize <= 0 {
		opt.ShardSize = 768
	}
	if opt.ShardSize > MaxChunkDataSize {
		return nil, fmt.Errorf("%w: shard size too large", ErrChunkInvalid)
	}
	dataShards := opt.DataShards
	if dataShards <= 0 {
		dataShards = (len(payload) + opt.ShardSize - 1) / opt.ShardSize
	}
	if dataShards <= 0 || dataShards > MaxShardCount {
		return nil, fmt.Errorf("%w: invalid data shard count", ErrChunkInvalid)
	}
	parityShards := opt.ParityShards
	if parityShards <= 0 {
		parityShards = (dataShards + 2) / 3
	}
	if parityShards <= 0 || dataShards+parityShards > MaxShardCount {
		return nil, fmt.Errorf("%w: invalid parity shard count", ErrChunkInvalid)
	}
	shardSize := opt.ShardSize
	minSize := (len(payload) + dataShards - 1) / dataShards
	if shardSize < minSize {
		shardSize = minSize
	}
	if shardSize > MaxChunkDataSize {
		return nil, fmt.Errorf("%w: shard size too large", ErrChunkInvalid)
	}

	sum := sha256.Sum256(payload)
	var id [16]byte
	copy(id[:], sum[:16])
	data := make([][]byte, dataShards)
	for i := 0; i < dataShards; i++ {
		data[i] = make([]byte, shardSize)
		start := i * shardSize
		if start >= len(payload) {
			continue
		}
		end := start + shardSize
		if end > len(payload) {
			end = len(payload)
		}
		copy(data[i], payload[start:end])
	}
	parity := make([][]byte, parityShards)
	for p := range parity {
		parity[p] = make([]byte, shardSize)
		row := codingRow(dataShards+p, dataShards)
		for col := 0; col < dataShards; col++ {
			coef := row[col]
			if coef == 0 {
				continue
			}
			for b := 0; b < shardSize; b++ {
				parity[p][b] ^= gfMul(coef, data[col][b])
			}
		}
	}

	total := dataShards + parityShards
	out := make([]EncodedChunk, 0, total)
	for i := 0; i < total; i++ {
		var shard []byte
		if i < dataShards {
			shard = data[i]
		} else {
			shard = parity[i-dataShards]
		}
		c := EncodedChunk{
			BundleID:      id,
			Index:         i,
			DataShards:    dataShards,
			ParityShards:  parityShards,
			PayloadSize:   len(payload),
			ShardSize:     shardSize,
			PayloadSHA256: sum,
			Data:          append([]byte(nil), shard...),
		}
		c.ChunkSHA256 = c.computeChecksum()
		out = append(out, c)
	}
	return out, nil
}

func (c EncodedChunk) MarshalBinary() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(make([]byte, 0, ChunkHeaderSize+len(c.Data)))
	buf.WriteString(ChunkMagic)
	buf.WriteByte(ChunkVersion)
	buf.WriteByte(0)
	buf.Write(c.BundleID[:])
	putU16Chunk(buf, c.Index)
	putU16Chunk(buf, c.DataShards)
	putU16Chunk(buf, c.ParityShards)
	putU32Chunk(buf, c.PayloadSize)
	putU16Chunk(buf, c.ShardSize)
	buf.Write(c.PayloadSHA256[:])
	buf.Write(c.ChunkSHA256[:])
	buf.Write(c.Data)
	return buf.Bytes(), nil
}

func ParseEncodedChunk(raw []byte) (EncodedChunk, error) {
	if len(raw) < ChunkHeaderSize {
		return EncodedChunk{}, fmt.Errorf("%w: short chunk", ErrChunkInvalid)
	}
	if string(raw[:4]) != ChunkMagic || raw[4] != ChunkVersion {
		return EncodedChunk{}, fmt.Errorf("%w: unsupported version", ErrChunkInvalid)
	}
	var c EncodedChunk
	copy(c.BundleID[:], raw[6:22])
	c.Index = int(binary.BigEndian.Uint16(raw[22:24]))
	c.DataShards = int(binary.BigEndian.Uint16(raw[24:26]))
	c.ParityShards = int(binary.BigEndian.Uint16(raw[26:28]))
	c.PayloadSize = int(binary.BigEndian.Uint32(raw[28:32]))
	c.ShardSize = int(binary.BigEndian.Uint16(raw[32:34]))
	copy(c.PayloadSHA256[:], raw[34:66])
	copy(c.ChunkSHA256[:], raw[66:98])
	c.Data = append([]byte(nil), raw[98:]...)
	if err := c.Validate(); err != nil {
		return EncodedChunk{}, err
	}
	if c.computeChecksum() != c.ChunkSHA256 {
		return EncodedChunk{}, ErrChunkChecksum
	}
	return c, nil
}

func (c EncodedChunk) EncodeText() (string, error) {
	raw, err := c.MarshalBinary()
	if err != nil {
		return "", err
	}
	return "SNBEC/1 " + base64.RawURLEncoding.EncodeToString(raw), nil
}

func ParseEncodedChunkText(line string) (EncodedChunk, error) {
	const prefix = "SNBEC/1 "
	if len(line) <= len(prefix) || line[:len(prefix)] != prefix {
		return EncodedChunk{}, fmt.Errorf("%w: invalid text envelope", ErrChunkInvalid)
	}
	raw, err := base64.RawURLEncoding.DecodeString(line[len(prefix):])
	if err != nil {
		return EncodedChunk{}, fmt.Errorf("%w: invalid base64", ErrChunkInvalid)
	}
	return ParseEncodedChunk(raw)
}

func (c EncodedChunk) Validate() error {
	total := c.DataShards + c.ParityShards
	if c.DataShards <= 0 || c.ParityShards <= 0 || total > MaxShardCount {
		return fmt.Errorf("%w: invalid shard counts", ErrChunkInvalid)
	}
	if c.Index < 0 || c.Index >= total {
		return fmt.Errorf("%w: invalid shard index", ErrChunkInvalid)
	}
	if c.PayloadSize <= 0 || c.PayloadSize > MaxBundleBytes {
		return fmt.Errorf("%w: invalid payload size", ErrChunkInvalid)
	}
	if c.ShardSize <= 0 || c.ShardSize > MaxChunkDataSize {
		return fmt.Errorf("%w: invalid shard size", ErrChunkInvalid)
	}
	if len(c.Data) != c.ShardSize {
		return fmt.Errorf("%w: invalid shard data size", ErrChunkInvalid)
	}
	if c.DataShards*c.ShardSize < c.PayloadSize {
		return fmt.Errorf("%w: shard matrix too small", ErrChunkInvalid)
	}
	return nil
}

func ReconstructErasurePayload(chunks []EncodedChunk) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("%w: no chunks", ErrChunkInvalid)
	}
	base := chunks[0]
	if err := base.Validate(); err != nil {
		return nil, err
	}
	byIndex := make(map[int]EncodedChunk, len(chunks))
	for _, c := range chunks {
		if err := c.Validate(); err != nil {
			return nil, err
		}
		if c.BundleID != base.BundleID || c.DataShards != base.DataShards || c.ParityShards != base.ParityShards ||
			c.PayloadSize != base.PayloadSize || c.ShardSize != base.ShardSize || c.PayloadSHA256 != base.PayloadSHA256 {
			return nil, fmt.Errorf("%w: mixed chunk set", ErrChunkInvalid)
		}
		if c.computeChecksum() != c.ChunkSHA256 {
			return nil, ErrChunkChecksum
		}
		byIndex[c.Index] = c
	}
	if len(byIndex) < base.DataShards {
		return nil, fmt.Errorf("%w: need %d chunks, have %d", ErrChunkUnrecoverable, base.DataShards, len(byIndex))
	}
	indices := make([]int, 0, len(byIndex))
	for idx := range byIndex {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	indices = indices[:base.DataShards]

	matrix := make([][]byte, base.DataShards)
	shards := make([][]byte, base.DataShards)
	for row, idx := range indices {
		matrix[row] = codingRow(idx, base.DataShards)
		shards[row] = byIndex[idx].Data
	}
	inv, err := invertMatrix(matrix)
	if err != nil {
		return nil, err
	}
	data := make([][]byte, base.DataShards)
	for i := 0; i < base.DataShards; i++ {
		data[i] = make([]byte, base.ShardSize)
		for row := 0; row < base.DataShards; row++ {
			coef := inv[i][row]
			if coef == 0 {
				continue
			}
			for b := 0; b < base.ShardSize; b++ {
				data[i][b] ^= gfMul(coef, shards[row][b])
			}
		}
	}
	payload := make([]byte, 0, base.DataShards*base.ShardSize)
	for i := 0; i < base.DataShards; i++ {
		payload = append(payload, data[i]...)
	}
	payload = payload[:base.PayloadSize]
	sum := sha256.Sum256(payload)
	if sum != base.PayloadSHA256 {
		return nil, fmt.Errorf("%w: payload digest mismatch", ErrChunkUnrecoverable)
	}
	if !bytes.Equal(sum[:16], base.BundleID[:]) {
		return nil, fmt.Errorf("%w: bundle id mismatch", ErrChunkUnrecoverable)
	}
	return payload, nil
}

type ChunkReassembler struct {
	mu         sync.Mutex
	partials   map[[16]byte]*chunkPartial
	maxBytes   int
	staleAfter time.Duration
}

type chunkPartial struct {
	meta      EncodedChunk
	chunks    map[int]EncodedChunk
	bytes     int
	updatedAt time.Time
}

func NewChunkReassembler(maxBytes int, staleAfter time.Duration) *ChunkReassembler {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxCacheByte
	}
	if staleAfter <= 0 {
		staleAfter = 10 * time.Minute
	}
	return &ChunkReassembler{
		partials:   make(map[[16]byte]*chunkPartial),
		maxBytes:   maxBytes,
		staleAfter: staleAfter,
	}
}

func (r *ChunkReassembler) Add(raw []byte, now time.Time) ([]byte, bool, error) {
	c, err := ParseEncodedChunk(raw)
	if err != nil {
		return nil, false, err
	}
	return r.AddChunk(c, now)
}

func (r *ChunkReassembler) AddChunk(c EncodedChunk, now time.Time) ([]byte, bool, error) {
	if now.IsZero() {
		now = time.Now()
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.partials[c.BundleID]
	if !ok {
		p = &chunkPartial{meta: c, chunks: make(map[int]EncodedChunk), updatedAt: now}
		r.partials[c.BundleID] = p
	}
	if c.BundleID != p.meta.BundleID || c.DataShards != p.meta.DataShards || c.ParityShards != p.meta.ParityShards ||
		c.PayloadSize != p.meta.PayloadSize || c.ShardSize != p.meta.ShardSize || c.PayloadSHA256 != p.meta.PayloadSHA256 {
		delete(r.partials, c.BundleID)
		return nil, false, fmt.Errorf("%w: header mismatch", ErrChunkInvalid)
	}
	if _, dup := p.chunks[c.Index]; !dup {
		if p.bytes+len(c.Data)+ChunkHeaderSize > r.maxBytes {
			delete(r.partials, c.BundleID)
			return nil, false, ErrChunkCacheLimit
		}
		p.chunks[c.Index] = c
		p.bytes += len(c.Data) + ChunkHeaderSize
	}
	p.updatedAt = now
	if len(p.chunks) < p.meta.DataShards {
		return nil, false, nil
	}
	list := make([]EncodedChunk, 0, len(p.chunks))
	for _, ch := range p.chunks {
		list = append(list, ch)
	}
	payload, err := ReconstructErasurePayload(list)
	delete(r.partials, c.BundleID)
	if err != nil {
		return nil, false, err
	}
	return payload, true, nil
}

func (r *ChunkReassembler) Sweep(now time.Time) int {
	if now.IsZero() {
		now = time.Now()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for id, p := range r.partials {
		if now.Sub(p.updatedAt) >= r.staleAfter {
			delete(r.partials, id)
			removed++
		}
	}
	return removed
}

func (c EncodedChunk) computeChecksum() [32]byte {
	buf := bytes.NewBuffer(make([]byte, 0, ChunkHeaderSize-32+len(c.Data)))
	buf.WriteString(ChunkMagic)
	buf.WriteByte(ChunkVersion)
	buf.WriteByte(0)
	buf.Write(c.BundleID[:])
	putU16Chunk(buf, c.Index)
	putU16Chunk(buf, c.DataShards)
	putU16Chunk(buf, c.ParityShards)
	putU32Chunk(buf, c.PayloadSize)
	putU16Chunk(buf, c.ShardSize)
	buf.Write(c.PayloadSHA256[:])
	buf.Write(c.Data)
	return sha256.Sum256(buf.Bytes())
}

func codingRow(index int, n int) []byte {
	row := make([]byte, n)
	if index < n {
		row[index] = 1
		return row
	}
	x := byte(index - n + 1)
	for col := 0; col < n; col++ {
		y := byte(128 + col)
		row[col] = gfInv(x ^ y)
	}
	return row
}

func invertMatrix(in [][]byte) ([][]byte, error) {
	n := len(in)
	if n == 0 {
		return nil, fmt.Errorf("%w: empty matrix", ErrChunkInvalid)
	}
	aug := make([][]byte, n)
	for i := 0; i < n; i++ {
		if len(in[i]) != n {
			return nil, fmt.Errorf("%w: non-square matrix", ErrChunkInvalid)
		}
		aug[i] = make([]byte, 2*n)
		copy(aug[i], in[i])
		aug[i][n+i] = 1
	}
	for col := 0; col < n; col++ {
		pivot := -1
		for row := col; row < n; row++ {
			if aug[row][col] != 0 {
				pivot = row
				break
			}
		}
		if pivot < 0 {
			return nil, fmt.Errorf("%w: singular matrix", ErrChunkUnrecoverable)
		}
		if pivot != col {
			aug[pivot], aug[col] = aug[col], aug[pivot]
		}
		invPivot := gfInv(aug[col][col])
		for j := col; j < 2*n; j++ {
			aug[col][j] = gfMul(aug[col][j], invPivot)
		}
		for row := 0; row < n; row++ {
			if row == col || aug[row][col] == 0 {
				continue
			}
			factor := aug[row][col]
			for j := col; j < 2*n; j++ {
				aug[row][j] ^= gfMul(factor, aug[col][j])
			}
		}
	}
	out := make([][]byte, n)
	for i := 0; i < n; i++ {
		out[i] = append([]byte(nil), aug[i][n:]...)
	}
	return out, nil
}

func gfAdd(a, b byte) byte { return a ^ b }

func gfMul(a, b byte) byte {
	var p byte
	for b != 0 {
		if b&1 != 0 {
			p = gfAdd(p, a)
		}
		hi := a & 0x80
		a <<= 1
		if hi != 0 {
			a ^= 0x1d
		}
		b >>= 1
	}
	return p
}

func gfPow(a byte, n int) byte {
	out := byte(1)
	for n > 0 {
		if n&1 == 1 {
			out = gfMul(out, a)
		}
		a = gfMul(a, a)
		n >>= 1
	}
	return out
}

func gfInv(a byte) byte {
	if a == 0 {
		return 0
	}
	return gfPow(a, 254)
}

func putU16Chunk(buf *bytes.Buffer, v int) {
	var tmp [2]byte
	binary.BigEndian.PutUint16(tmp[:], uint16(v))
	buf.Write(tmp[:])
}

func putU32Chunk(buf *bytes.Buffer, v int) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], uint32(v))
	buf.Write(tmp[:])
}
