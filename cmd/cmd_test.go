package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bizshuk/agentsdk/core"
	"github.com/bizshuk/agentsdk/provider"
	gosdkcmd "github.com/bizshuk/gosdk/cmd"
	cmdinternal "github.com/bizshuk/ocr_agent/cmd/internal"
	"github.com/bizshuk/ocr_agent/svc/task"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// stubAdapter answers every request with the mv args configured for its
// model, or no tool call when none are, and records which models were built.
type stubAdapter struct{ model string }

var (
	stubMu    sync.Mutex
	stubCalls = map[string]map[string]any{} // model -> mv args
	stubBuilt []string                      // models in build order
)

func (s stubAdapter) Generate(context.Context, core.ModelRequest) (core.ModelResult, error) {
	stubMu.Lock()
	defer stubMu.Unlock()
	args, ok := stubCalls[s.model]
	if !ok {
		return core.ModelResult{Text: "nothing fits"}, nil
	}
	return core.ModelResult{ToolCalls: []core.ToolCall{{ID: "1", Name: task.TOOL_NAME, Args: args}}}, nil
}

func (s stubAdapter) Stream(context.Context, core.ModelRequest) (<-chan core.ModelChunk, error) {
	ch := make(chan core.ModelChunk)
	close(ch)
	return ch, nil
}

func init() {
	provider.Register(provider.Entry{
		Name: "stub",
		New: func(cfg provider.ResolvedConfig) (provider.Adapter, error) {
			stubMu.Lock()
			stubBuilt = append(stubBuilt, cfg.Model)
			stubMu.Unlock()
			return stubAdapter{model: cfg.Model}, nil
		},
	})
}

// layout seeds viper with an inbox classify task and two date tasks under
// a temp dir and returns the dir.
func layout(t *testing.T) string {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	stubMu.Lock()
	stubCalls = map[string]map[string]any{}
	stubBuilt = nil
	stubMu.Unlock()

	root := t.TempDir()
	setTasks(
		taskEntry("inbox", task.TYPE_CLASSIFY, "router", filepath.Join(root, "inbox"),
			tgt("general_inbox", filepath.Join(root, "general", "tmp")),
			tgt("special", filepath.Join(root, "special"))),
		taskEntry("general", task.TYPE_DATE, "general-m", filepath.Join(root, "general", "tmp"),
			tgt("general_archive", filepath.Join(root, "general", "{yyyy}", "{mm}"))),
		taskEntry("medical", task.TYPE_DATE, "medical-m", filepath.Join(root, "medical", "tmp"),
			tgt("medical_archive", filepath.Join(root, "medical", "{yyyy}", "{mm}"))),
	)
	for _, dir := range []string{"inbox/nested", "general/tmp", "medical/tmp"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func taskEntry(name, typ, model, source string, targets ...map[string]any) map[string]any {
	return map[string]any{"name": name, "description": "describe " + name, "type": typ,
		"provider": "stub", "model": model, "timeout": "1m", "source": source, "targets": targets}
}

func tgt(name, path string) map[string]any {
	return map[string]any{"name": name, "description": name + " folder", "path": path}
}

func setTasks(tasks ...map[string]any) { viper.Set(cmdinternal.KEY_TASKS, tasks) }

func writeImage(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{0xff, 0xd8}, 0o600); err != nil {
		t.Fatal(err)
	}
}

// execute runs a type subcommand (or run, when typ is empty) the way its
// RunE does, with split stdout / stderr buffers.
func execute(t *testing.T, typ, source string, o cmdinternal.RunOptions) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetContext(context.Background())
	if typ == "" {
		tasks, loadErr := cmdinternal.LoadTasks()
		if loadErr != nil {
			return "", "", loadErr
		}
		err = cmdinternal.RunEach(c, tasks, o)
	} else {
		task, findErr := cmdinternal.FindTask(typ, source)
		if findErr != nil {
			return "", "", findErr
		}
		err = cmdinternal.RunTask(c, task, o)
	}
	return out.String(), errOut.String(), err
}

func TestSubcommandsAtRoot(t *testing.T) {
	for _, name := range []string{"config", task.TYPE_CLASSIFY, task.TYPE_DATE, "run"} {
		got, _, err := RootCmd.Find([]string{name})
		if err != nil || got == RootCmd {
			t.Fatalf("find %s: %v", name, err)
		}
	}
	if got, _, _ := RootCmd.Find([]string{"config"}); got != gosdkcmd.ConfigCmd {
		t.Fatal("config must be the gosdk config command")
	}
}

// classify --apply moves an image, found recursively, into the chosen folder.
func TestClassifyMovesIntoChosenFolder(t *testing.T) {
	root := layout(t)
	stubCalls["router"] = map[string]any{"target": "special", "metadata": map[string]any{"category": "letter"}}
	writeImage(t, filepath.Join(root, "inbox", "nested", "a.jpg"))

	out, errOut, err := execute(t, task.TYPE_CLASSIFY, "", cmdinternal.RunOptions{Apply: true})
	if err != nil {
		t.Fatalf("classify: %v\n%s%s", err, out, errOut)
	}
	if _, err := os.Stat(filepath.Join(root, "special", "a.jpg")); err != nil {
		t.Fatalf("image not moved to special: %v\n%s", err, out)
	}
}

// date --source <general inbox> --apply moves the image and its sidecar into
// the dated folder.
func TestDateMovesImageAndSidecarToDatedFolder(t *testing.T) {
	root := layout(t)
	stubCalls["general-m"] = map[string]any{"target": "general_archive", "metadata": map[string]any{"date": "2026-08-04"}}
	writeImage(t, filepath.Join(root, "general", "tmp", "c.jpg"))

	out, errOut, err := execute(t, task.TYPE_DATE, filepath.Join(root, "general", "tmp"), cmdinternal.RunOptions{Apply: true})
	if err != nil {
		t.Fatalf("date: %v\n%s%s", err, out, errOut)
	}
	for _, name := range []string{"c.jpg", "c.json"} {
		if _, err := os.Stat(filepath.Join(root, "general", "2026", "08", name)); err != nil {
			t.Fatalf("%s missing: %v\n%s", name, err, out)
		}
	}
}

// Per-image failures go to stderr; progress and the summary to stdout.
func TestFailuresGoToStderr(t *testing.T) {
	root := layout(t)
	stubCalls["general-m"] = map[string]any{"target": "general_archive", "metadata": map[string]any{"merchant": "no date"}}
	writeImage(t, filepath.Join(root, "general", "tmp", "bad.jpg"))

	out, errOut, err := execute(t, task.TYPE_DATE, filepath.Join(root, "general", "tmp"), cmdinternal.RunOptions{})
	if err == nil {
		t.Fatal("a failed image must fail the task")
	}
	if !strings.Contains(errOut, "bad.jpg") || strings.Contains(out, "bad.jpg") {
		t.Fatalf("failure must be on stderr only\nstdout:\n%s\nstderr:\n%s", out, errOut)
	}
	if !strings.Contains(out, "Errors: 1") {
		t.Fatalf("summary missing from stdout:\n%s", out)
	}
}

// A subcommand runs only the task --source picks; run walks every task.
func TestTaskSelection(t *testing.T) {
	root := layout(t)
	if _, _, err := execute(t, task.TYPE_DATE, filepath.Join(root, "medical", "tmp"), cmdinternal.RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(stubBuilt, ","); got != "medical-m" {
		t.Fatalf("date --source medical built %s, want only medical-m", got)
	}
	stubBuilt = nil
	if _, _, err := execute(t, "", "", cmdinternal.RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(stubBuilt, ","); got != "router,general-m,medical-m" {
		t.Fatalf("run built %s, want every task in config order", got)
	}
}

func TestSourceSelectsExactlyOneTask(t *testing.T) {
	root := layout(t)
	if task, err := cmdinternal.FindTask(task.TYPE_CLASSIFY, ""); err != nil || task.Name != "inbox" {
		t.Fatalf("the only classify task needs no --source: %v %v", task.Name, err)
	}
	cases := map[string]struct{ typ, source string }{
		"pick one with --source":  {task.TYPE_DATE, ""},
		"no date task has source": {task.TYPE_DATE, filepath.Join(root, "inbox")},
	}
	for want, c := range cases {
		if _, err := cmdinternal.FindTask(c.typ, c.source); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("FindTask(%s, %q) = %v, want %q", c.typ, c.source, err, want)
		}
	}
}

// A target inside a recursively scanned source would be classified again on
// every run, so the config is refused before any model call.
func TestClassifyRefusesTargetInsideSource(t *testing.T) {
	root := layout(t)
	setTasks(taskEntry("inbox", task.TYPE_CLASSIFY, "router", filepath.Join(root, "inbox"),
		tgt("loop", filepath.Join(root, "inbox", "general"))))
	if _, err := cmdinternal.LoadTasks(); err == nil || !strings.Contains(err.Error(), "must not lie inside") {
		t.Fatalf("got %v, want a target-inside-source error", err)
	}
}

func TestInvalidTasksNameTheProblem(t *testing.T) {
	root := layout(t)
	renamed := func(entry map[string]any, name string) map[string]any { entry["name"] = name; return entry }
	valid := func() map[string]any {
		return taskEntry("x", task.TYPE_DATE, "m", root, tgt("a", filepath.Join(root, "a")))
	}
	noDescription := valid()
	noDescription["description"] = " "
	badType := valid()
	badType["type"] = "general"
	noTargets := valid()
	delete(noTargets, "targets")
	cases := map[string][]map[string]any{
		"description is not set": {noDescription},
		"type must be one of":    {badType},
		"no targets":             {noTargets},
		"duplicate name":         {valid(), valid()},
		"already reads":          {valid(), renamed(valid(), "y")},
	}
	for want, entries := range cases {
		setTasks(entries...)
		if _, err := cmdinternal.LoadTasks(); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("got %v, want %q", err, want)
		}
	}
	viper.Reset()
	if _, err := cmdinternal.LoadTasks(); err == nil || !strings.Contains(err.Error(), "tasks is not set") {
		t.Fatalf("got %v, want tasks is not set", err)
	}
}
