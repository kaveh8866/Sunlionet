package sbctl

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/kaveh/shadownet-agent/pkg/profile"
)

// Controller manages the sing-box process and config generation
type Controller struct {
	ConfigDir  string
	BinaryPath string
	mu         sync.Mutex
	cmd        *exec.Cmd
}

// NewController creates a new sing-box controller
func NewController(configDir, binaryPath string) *Controller {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		log.Fatalf("Failed to create sbctl config dir: %v", err)
	}
	return &Controller{
		ConfigDir:  configDir,
		BinaryPath: binaryPath,
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

	// Atomic write: write to temp file, then rename
	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(configJSON), 0600); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	// Basic validation could happen here before renaming
	// e.g. running `sing-box check -c tempPath`

	if err := os.Rename(tempPath, configPath); err != nil {
		return fmt.Errorf("failed to commit config: %w", err)
	}

	// If sing-box is running, try to reload. Otherwise start it.
	if c.cmd != nil && c.cmd.Process != nil {
		log.Println("sbctl: Reloading sing-box config...")
		// In a real scenario, sing-box supports hot-reloading via SIGHUP on Linux
		// or an API call. For simplicity here, we'll restart the process.
		if err := c.cmd.Process.Kill(); err != nil {
			log.Printf("sbctl: Warning: failed to kill old process: %v", err)
		}
		c.cmd.Wait()
	}

	log.Println("sbctl: Starting sing-box...")
	// We mock the actual execution if the binary doesn't exist for local dev
	if _, err := os.Stat(c.BinaryPath); os.IsNotExist(err) {
		log.Printf("sbctl: [MOCK] sing-box binary not found at %s, simulating run", c.BinaryPath)
		return nil
	}

	c.cmd = exec.Command(c.BinaryPath, "run", "-c", configPath)
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start sing-box: %w", err)
	}

	return nil
}

// Stop gracefully shuts down sing-box
func (c *Controller) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil && c.cmd.Process != nil {
		log.Println("sbctl: Stopping sing-box...")
		if err := c.cmd.Process.Kill(); err != nil {
			return err
		}
		c.cmd.Wait()
		c.cmd = nil
	}
	return nil
}
