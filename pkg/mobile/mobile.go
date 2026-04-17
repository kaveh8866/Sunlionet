package mobile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kaveh/shadownet-agent/pkg/mobilebridge"
	"github.com/kaveh/shadownet-agent/pkg/profile"
)

func StartAgent(config string) error {
	var cfg mobilebridge.AgentConfig
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if err := validateConfig(&cfg); err != nil {
		return err
	}
	mobilebridge.StartAgent(config)
	return nil
}

func StopAgent() error {
	mobilebridge.StopAgent()
	return nil
}

func ImportBundle(path string) error {
	return mobilebridge.ImportBundle(path)
}

func GetStatus() string {
	return mobilebridge.GetStatus()
}

func validateConfig(cfg *mobilebridge.AgentConfig) error {
	if strings.TrimSpace(cfg.StateDir) == "" {
		return fmt.Errorf("missing state_dir")
	}
	if _, err := profile.ParseMasterKey(cfg.MasterKey); err != nil {
		return fmt.Errorf("invalid master_key: %w", err)
	}
	if strings.TrimSpace(cfg.TemplatesDir) == "" {
		return fmt.Errorf("missing templates_dir")
	}
	if cfg.PollIntervalSec <= 0 {
		cfg.PollIntervalSec = 20
	}
	if cfg.PollIntervalSec < 10 {
		cfg.PollIntervalSec = 10
	}
	if cfg.PollIntervalSec > 300 {
		cfg.PollIntervalSec = 300
	}
	if cfg.PiTimeoutMS <= 0 {
		cfg.PiTimeoutMS = 1200
	}
	if strings.TrimSpace(cfg.PiCommand) == "" {
		cfg.PiCommand = "pi"
	}
	return nil
}
