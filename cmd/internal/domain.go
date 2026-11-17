package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/agentsdk/provider"
	_ "github.com/bizshuk/agentsdk/provider/all" // link every adapter; selection is config, not code
	"github.com/bizshuk/gosdk/config"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// APP_NAME anchors the gosdk config directory: ~/.config/ocr_agent.
const APP_NAME = "ocr_agent"

// The three domains. Each one owns a config block at domain.<name>.
const (
	DOMAIN_CLASSIFY = "classify"
	DOMAIN_GENERAL  = "general"
	DOMAIN_MEDICAL  = "medical"
)

// Field names inside a domain.<name> block.
const (
	FIELD_PROVIDER = "provider"
	FIELD_MODEL    = "model"
	FIELD_BASE_URL = "base_url"
	FIELD_TIMEOUT  = "timeout"
	FIELD_SOURCE   = "source"
	FIELD_TARGET   = "target"
)

// DEFAULT_TIMEOUT is the per-image HTTP timeout when a domain sets none. A
// local vision model reading a multi-megabyte photo routinely needs minutes.
const DEFAULT_TIMEOUT = 10 * time.Minute

// Domain is one fully resolved domain block. Paths are absolute.
type Domain struct {
	Name     string
	Provider string
	Model    string
	BaseURL  string
	Timeout  time.Duration
	Source   string
	Target   string
}

// ModelOverride carries the --provider / --model flags a single run may use
// to replace the domain's configured model binding.
type ModelOverride struct {
	Provider string
	Model    string
}

// BindModelOverride adds the shared model flags. There is deliberately no
// --api-key: argv is world-readable through ps, so credentials come from
// config or the environment only.
func BindModelOverride(flags *pflag.FlagSet, o *ModelOverride) {
	flags.StringVar(&o.Provider, "provider", "", "override domain.<name>.provider for this run (e.g. ollama, minimax)")
	flags.StringVar(&o.Model, "model", "", "override domain.<name>.model for this run")
}

// DomainKey returns the viper config key for a domain field.
func DomainKey(name, field string) string {
	return "domain." + name + "." + field
}

// LoadDomain reads domain.<name> from config. Only source is required here;
// the model binding is checked by NewProvider and target by RequireTarget,
// so a domain read just for its source does not need a complete block.
func LoadDomain(name string, o ModelOverride) (Domain, error) {
	d := Domain{
		Name:     name,
		Provider: firstNonEmpty(o.Provider, viper.GetString(DomainKey(name, FIELD_PROVIDER))),
		Model:    firstNonEmpty(o.Model, viper.GetString(DomainKey(name, FIELD_MODEL))),
		BaseURL:  viper.GetString(DomainKey(name, FIELD_BASE_URL)),
		Timeout:  viper.GetDuration(DomainKey(name, FIELD_TIMEOUT)),
	}
	if d.Timeout <= 0 {
		d.Timeout = DEFAULT_TIMEOUT
	}
	source, err := resolvePath(name, FIELD_SOURCE)
	if err != nil {
		return Domain{}, err
	}
	if source == "" {
		return Domain{}, missingKey(name, FIELD_SOURCE)
	}
	d.Source = source
	if d.Target, err = resolvePath(name, FIELD_TARGET); err != nil {
		return Domain{}, err
	}
	return d, nil
}

// RequireTarget fails when an extract domain has no archive target.
func (d Domain) RequireTarget() error {
	if d.Target == "" {
		return missingKey(d.Name, FIELD_TARGET)
	}
	return nil
}

// NewProvider resolves the domain's provider name through the agentsdk
// registry. Adapters self-register from init(), so pointing a domain at a
// different vendor is a config edit, never a code change. An empty base_url
// lets the registry consult the adapter's own env var and default endpoint.
func (d Domain) NewProvider() (core.Provider, error) {
	if d.Provider == "" {
		return nil, missingKey(d.Name, FIELD_PROVIDER)
	}
	if d.Model == "" {
		return nil, missingKey(d.Name, FIELD_MODEL)
	}
	adapter, err := provider.New(d.Provider, provider.Options{
		Model:     d.Model,
		BaseURL:   d.BaseURL,
		Timeout:   d.Timeout,
		LookupEnv: lookupEnv,
	})
	if err != nil {
		return nil, fmt.Errorf("domain %s: build %s provider: %w", d.Name, d.Provider, err)
	}
	return adapter, nil
}

// String renders the domain for stage headers: "general minimax:MiniMax-M3".
func (d Domain) String() string {
	return d.Name + " " + d.Provider + ":" + d.Model
}

func resolvePath(name, field string) (string, error) {
	raw := viper.GetString(DomainKey(name, field))
	if raw == "" {
		return "", nil
	}
	abs, err := filepath.Abs(config.ExpandHome(raw))
	if err != nil {
		return "", fmt.Errorf("%s: %w", DomainKey(name, field), err)
	}
	return abs, nil
}

func missingKey(name, field string) error {
	return fmt.Errorf("%s is not set (seed it with `%s config default`)", DomainKey(name, field), APP_NAME)
}

// lookupEnv lets the registry read credentials and endpoints through viper
// first, so a value in ~/.config/ocr_agent/.env participates, then falls
// back to the process environment.
func lookupEnv(key string) string {
	if value := viper.GetString(key); value != "" {
		return value
	}
	return os.Getenv(key)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ErrFailed is the batch-level error returned after per-file failures were
// already printed, so pm2 sees a non-zero exit.
func ErrFailed(n int) error {
	if n == 0 {
		return nil
	}
	return fmt.Errorf("%d image(s) failed", n)
}
