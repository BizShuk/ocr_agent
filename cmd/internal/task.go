package internal

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/agentsdk/provider"
	_ "github.com/bizshuk/agentsdk/provider/all" // link every adapter; selection is config, not code
	"github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/ocr_agent/svc/filing"
	"github.com/bizshuk/ocr_agent/svc/target"
	"github.com/bizshuk/ocr_agent/svc/task"
	"github.com/spf13/viper"
)

// APP_NAME anchors the gosdk config directory: ~/.config/ocr_agent.
const APP_NAME = "ocr_agent"

// KEY_TASKS is the config key of the task list.
const KEY_TASKS = "tasks"

// DEFAULT_TIMEOUT is the per-request HTTP timeout when a task sets none. A
// vision model reading a multi-megabyte photo routinely needs minutes.
const DEFAULT_TIMEOUT = 10 * time.Minute

// Task is one tasks[] object, decoded straight from settings.json. After
// LoadTasks its paths are absolute and Contract is set from Type.
type Task struct {
	Name        string          `mapstructure:"name"`
	Description string          `mapstructure:"description"`
	Type        string          `mapstructure:"type"`
	Provider    string          `mapstructure:"provider"`
	Model       string          `mapstructure:"model"`
	BaseURL     string          `mapstructure:"base_url"`
	Timeout     time.Duration   `mapstructure:"timeout"`
	Source      string          `mapstructure:"source"`
	Targets     []target.Target `mapstructure:"targets"`

	Contract task.Contract `mapstructure:"-"`
}

// LoadTasks reads and validates every tasks[] object, in config order.
func LoadTasks() ([]Task, error) {
	if !viper.IsSet(KEY_TASKS) {
		return nil, fmt.Errorf("%s is not set (seed it with `%s config default`)", KEY_TASKS, APP_NAME)
	}
	var tasks []Task
	if err := viper.UnmarshalKey(KEY_TASKS, &tasks); err != nil {
		return nil, fmt.Errorf("%s: %w", KEY_TASKS, err)
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("%s is empty", KEY_TASKS)
	}
	names, sourcesByType := map[string]bool{}, map[string]bool{}
	for i := range tasks {
		t := &tasks[i]
		err := t.resolve()
		switch {
		case err != nil:
		case names[t.Name]:
			err = errors.New("duplicate name")
		case sourcesByType[t.Type+"\x00"+t.Source]:
			err = fmt.Errorf("another %s task already reads %s; --source could not tell them apart", t.Type, t.Source)
		}
		if err != nil {
			return nil, fmt.Errorf("%s[%d] %q: %w", KEY_TASKS, i, t.Name, err)
		}
		names[t.Name], sourcesByType[t.Type+"\x00"+t.Source] = true, true
	}
	return tasks, nil
}

// FindTask returns the one task of type typ whose source is source. An
// empty source is allowed only when the type has exactly one task.
func FindTask(typ, source string) (Task, error) {
	all, err := LoadTasks()
	if err != nil {
		return Task{}, err
	}
	var candidates []Task
	for _, t := range all {
		if t.Type == typ {
			candidates = append(candidates, t)
		}
	}
	if source == "" {
		if len(candidates) == 1 {
			return candidates[0], nil
		}
		return Task{}, fmt.Errorf("%d %s tasks configured: pick one with --source (%s)", len(candidates), typ, strings.Join(sources(candidates), ", "))
	}
	abs, err := absPath(source)
	if err != nil {
		return Task{}, fmt.Errorf("--source: %w", err)
	}
	for _, t := range candidates {
		if t.Source == abs {
			return t, nil
		}
	}
	return Task{}, fmt.Errorf("no %s task has source %s (configured: %s)", typ, abs, strings.Join(sources(candidates), ", "))
}

func sources(tasks []Task) []string {
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.Source)
	}
	return out
}

// resolve validates the task in place: required fields, a known type,
// absolute paths, and targets the model can choose from.
func (t *Task) resolve() error {
	for _, f := range []struct{ key, value string }{
		{"name", t.Name}, {"description", t.Description}, {"type", t.Type},
		{"provider", t.Provider}, {"model", t.Model}, {"source", t.Source},
	} {
		if strings.TrimSpace(f.value) == "" {
			return fmt.Errorf("%s is not set", f.key)
		}
	}
	contract, ok := task.Lookup(t.Type)
	if !ok {
		return fmt.Errorf("type must be one of %v, got %q", task.Types(), t.Type)
	}
	t.Contract = contract
	if t.Timeout <= 0 {
		t.Timeout = DEFAULT_TIMEOUT
	}
	var err error
	if t.Source, err = absPath(t.Source); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	for i := range t.Targets {
		if t.Targets[i].Path, err = absPath(t.Targets[i].Path); err != nil {
			return fmt.Errorf("target %q: %w", t.Targets[i].Name, err)
		}
	}
	if err := target.Validate(t.Targets); err != nil {
		return fmt.Errorf("targets: %w", err)
	}
	if contract.Recursive {
		return refuseTargetsInside(t.Source, t.Targets)
	}
	return nil
}

// refuseTargetsInside rejects a target under a recursively scanned source:
// a file filed there would be picked up again on every run.
func refuseTargetsInside(source string, targets []target.Target) error {
	for _, t := range targets {
		inside, err := filing.Within(source, t.Root())
		if err != nil {
			return err
		}
		if inside {
			return fmt.Errorf("target %q (%s) must not lie inside the recursively scanned source %s", t.Name, t.Path, source)
		}
	}
	return nil
}

// NewProvider resolves the task's provider name through the agentsdk
// registry, so pointing a task at another vendor is a config edit. Viper
// answers credential lookups: gosdk binds the environment and
// ~/.config/ocr_agent/.env into it.
func (t Task) NewProvider() (core.Provider, error) {
	adapter, err := provider.New(t.Provider, provider.Options{
		Model:     t.Model,
		BaseURL:   t.BaseURL,
		Timeout:   t.Timeout,
		LookupEnv: viper.GetString,
	})
	if err != nil {
		return nil, fmt.Errorf("task %s: build %s provider: %w", t.Name, t.Provider, err)
	}
	return adapter, nil
}

// String renders the task for headers: "medical (date) minimax:MiniMax-M3".
func (t Task) String() string {
	return t.Name + " (" + t.Type + ") " + t.Provider + ":" + t.Model
}

// absPath expands ~ and makes raw absolute; empty stays empty.
func absPath(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	return filepath.Abs(config.ExpandHome(raw))
}
