package orchestrator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"
	"time"
)

var ErrUnavailable = errors.New("orchestrator unavailable")

type Client interface {
	Decide(ctx context.Context, req DecisionRequest) (DecisionResponse, error)
	Close() error
}

type Config struct {
	UseTCP   bool
	Endpoint string
	Command  string
	Args     []string
	Timeout  time.Duration
}

func NewClient(cfg Config) (Client, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 1200 * time.Millisecond
	}
	if cfg.UseTCP {
		if cfg.Endpoint == "" {
			return nil, fmt.Errorf("%w: missing endpoint", ErrUnavailable)
		}
		return newTCPClient(cfg.Endpoint), nil
	}
	if cfg.Command == "" {
		cfg.Command = "pi"
	}
	return newStdioProcessClient(cfg.Command, cfg.Args)
}

type stdioProcessClient struct {
	cmd  *exec.Cmd
	in   io.WriteCloser
	out  *bufio.Reader
	mu   sync.Mutex
	next uint64
}

func newStdioProcessClient(command string, args []string) (*stdioProcessClient, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: stdin pipe: %v", ErrUnavailable, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("%w: stdout pipe: %v", ErrUnavailable, err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("%w: start %q: %v", ErrUnavailable, command, err)
	}
	return &stdioProcessClient{
		cmd: cmd,
		in:  stdin,
		out: bufio.NewReader(stdout),
	}, nil
}

func (c *stdioProcessClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.in != nil {
		_ = c.in.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return nil
}

func (c *stdioProcessClient) Decide(ctx context.Context, req DecisionRequest) (DecisionResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.next++
	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.next,
		Method:  "decide",
		Params:  req,
	}

	line, err := json.Marshal(rpcReq)
	if err != nil {
		return DecisionResponse{}, err
	}
	line = append(line, '\n')

	if err := writeWithContext(ctx, c.in, line); err != nil {
		return DecisionResponse{}, fmt.Errorf("%w: write: %v", ErrUnavailable, err)
	}

	respLine, err := readLineWithContext(ctx, c.out)
	if err != nil {
		return DecisionResponse{}, fmt.Errorf("%w: read: %v", ErrUnavailable, err)
	}

	var rpcResp JSONRPCResponse
	if err := decodeStrict(respLine, &rpcResp); err != nil {
		return DecisionResponse{}, fmt.Errorf("invalid rpc response: %w", err)
	}
	if rpcResp.JSONRPC != "2.0" || rpcResp.ID != rpcReq.ID {
		return DecisionResponse{}, fmt.Errorf("invalid rpc envelope")
	}
	if rpcResp.Error != nil {
		return DecisionResponse{}, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

type tcpClient struct {
	conn net.Conn
	r    *bufio.Reader
	w    *bufio.Writer
	mu   sync.Mutex
	next uint64
}

func newTCPClient(endpoint string) *tcpClient {
	return &tcpClient{conn: &lazyConn{endpoint: endpoint}}
}

func (c *tcpClient) ensureConnected(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("%w: no conn", ErrUnavailable)
	}
	if lc, ok := c.conn.(*lazyConn); ok && lc.conn == nil {
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", lc.endpoint)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		lc.conn = conn
	}
	if lc, ok := c.conn.(*lazyConn); ok && lc.conn != nil {
		c.r = bufio.NewReader(lc.conn)
		c.w = bufio.NewWriter(lc.conn)
	}
	return nil
}

func (c *tcpClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if lc, ok := c.conn.(*lazyConn); ok && lc.conn != nil {
		return lc.conn.Close()
	}
	return nil
}

func (c *tcpClient) Decide(ctx context.Context, req DecisionRequest) (DecisionResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureConnected(ctx); err != nil {
		return DecisionResponse{}, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		if lc, ok := c.conn.(*lazyConn); ok && lc.conn != nil {
			_ = lc.conn.SetDeadline(deadline)
		}
	}
	c.next++
	rpcReq := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.next,
		Method:  "decide",
		Params:  req,
	}
	b, err := json.Marshal(rpcReq)
	if err != nil {
		return DecisionResponse{}, err
	}
	b = append(b, '\n')
	if err := writeWithContext(ctx, c.w, b); err != nil {
		return DecisionResponse{}, fmt.Errorf("%w: write: %v", ErrUnavailable, err)
	}
	if err := c.w.Flush(); err != nil {
		return DecisionResponse{}, fmt.Errorf("%w: flush: %v", ErrUnavailable, err)
	}
	respLine, err := readLineWithContext(ctx, c.r)
	if err != nil {
		return DecisionResponse{}, fmt.Errorf("%w: read: %v", ErrUnavailable, err)
	}
	var rpcResp JSONRPCResponse
	if err := decodeStrict(respLine, &rpcResp); err != nil {
		return DecisionResponse{}, fmt.Errorf("invalid rpc response: %w", err)
	}
	if rpcResp.JSONRPC != "2.0" || rpcResp.ID != rpcReq.ID {
		return DecisionResponse{}, fmt.Errorf("invalid rpc envelope")
	}
	if rpcResp.Error != nil {
		return DecisionResponse{}, fmt.Errorf("rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

type lazyConn struct {
	endpoint string
	conn     net.Conn
}

func (l *lazyConn) Read(p []byte) (int, error)  { return l.conn.Read(p) }
func (l *lazyConn) Write(p []byte) (int, error) { return l.conn.Write(p) }
func (l *lazyConn) Close() error                { return l.conn.Close() }
func (l *lazyConn) LocalAddr() net.Addr         { return l.conn.LocalAddr() }
func (l *lazyConn) RemoteAddr() net.Addr        { return l.conn.RemoteAddr() }
func (l *lazyConn) SetDeadline(t time.Time) error {
	return l.conn.SetDeadline(t)
}
func (l *lazyConn) SetReadDeadline(t time.Time) error {
	return l.conn.SetReadDeadline(t)
}
func (l *lazyConn) SetWriteDeadline(t time.Time) error {
	return l.conn.SetWriteDeadline(t)
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

func writeWithContext(ctx context.Context, w io.Writer, b []byte) error {
	type res struct {
		err error
	}
	ch := make(chan res, 1)
	go func() {
		_, err := w.Write(b)
		ch <- res{err: err}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-ch:
		return r.err
	}
}

func readLineWithContext(ctx context.Context, r *bufio.Reader) ([]byte, error) {
	type res struct {
		line []byte
		err  error
	}
	ch := make(chan res, 1)
	go func() {
		line, err := r.ReadBytes('\n')
		if err != nil {
			ch <- res{err: err}
			return
		}
		ch <- res{line: bytes.TrimSpace(line)}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.line, r.err
	}
}
