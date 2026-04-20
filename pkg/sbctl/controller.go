package sbctl

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"text/template"

	"github.com/kaveh/sunlionet-agent/pkg/profile"
)

var ErrBinaryNotFound = errors.New("sing-box binary not found")

// Controller manages the sing-box process and config generation
type Controller struct {
	ConfigDir  string
	BinaryPath string
	mu         sync.Mutex
	cmd        *exec.Cmd
	stdoutPath string
	stderrPath string
}

// NewController creates a new sing-box controller
func NewController(configDir, binaryPath string) *Controller {
	_ = os.MkdirAll(configDir, 0o700)
	return &Controller{
		ConfigDir:  configDir,
		BinaryPath: binaryPath,
		stdoutPath: filepath.Join(configDir, "sing-box.stdout.log"),
		stderrPath: filepath.Join(configDir, "sing-box.stderr.log"),
	}
}

// GenerateConfig combines a template with profile data
func (c *Controller) GenerateConfig(prof profile.Profile, templateText string) (string, error) {
	tmpl, err := template.New("sbconfig").Parse(templateText)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	// Pass the whole profile to the template. The template can access .Endpoint.Host, etc.
	if err := tmpl.Execute(&buf, prof); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// ApplyAndReload writes the config to disk and hot-reloads or restarts sing-box
func (c *Controller) ApplyAndReload(configJSON string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	configPath := filepath.Join(c.ConfigDir, "config.json")

	if err := writeAtomic(configPath, []byte(configJSON), 0o600); err != nil {
		return err
	}

	if err := c.ValidateConfig(configPath); err != nil {
		return err
	}

	return c.reloadOrRestartLocked(configPath)
}

// Stop gracefully shuts down sing-box
func (c *Controller) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			return err
		}
		c.cmd.Wait()
		c.cmd = nil
	}
	return nil
}

func (c *Controller) PID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cmd == nil || c.cmd.Process == nil {
		return 0
	}
	return c.cmd.Process.Pid
}

func (c *Controller) StdoutPath() string {
	return c.stdoutPath
}

func (c *Controller) StderrPath() string {
	return c.stderrPath
}

func (c *Controller) ValidateConfig(configPath string) error {
	bin := c.BinaryPath
	if bin == "" {
		looked, err := exec.LookPath("sing-box")
		if err != nil {
			return ErrBinaryNotFound
		}
		bin = looked
	}
	if _, err := os.Stat(bin); err != nil {
		return ErrBinaryNotFound
	}
	cmd := exec.Command(bin, "check", "-c", configPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sing-box check failed: %w: %s", err, string(out))
	}
	return nil
}

func (c *Controller) reloadOrRestartLocked(configPath string) error {
	bin := c.BinaryPath
	if bin == "" {
		looked, err := exec.LookPath("sing-box")
		if err != nil {
			return ErrBinaryNotFound
		}
		bin = looked
	}
	if _, err := os.Stat(bin); err != nil {
		return ErrBinaryNotFound
	}

	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Signal(syscall.SIGHUP)
		return nil
	}

	stdoutFile, _ := os.OpenFile(c.stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	stderrFile, _ := os.OpenFile(c.stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)

	c.cmd = exec.Command(bin, "run", "-c", configPath)
	c.cmd.Stdout = io.MultiWriter(os.Stdout, stdoutFile)
	c.cmd.Stderr = io.MultiWriter(os.Stderr, stderrFile)
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start sing-box: %w", err)
	}
	return nil
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("failed to commit config: %w", err)
	}
	return nil
}
