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
	receiptcmd "github.com/bizshuk/ocr_agent/cmd/receipt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// stubAdapter answers every request with a canned reply and records which
// model it was built for, so a test can tell the domains' providers apart.
type stubAdapter struct{ model string }

var (
	stubMu      sync.Mutex
	stubReplies = map[string]string{} // model -> reply
	stubBuilt   []string              // models in build order
)

func (s stubAdapter) Generate(context.Context, core.ModelRequest) (core.ModelResult, error) {
	stubMu.Lock()
	defer stubMu.Unlock()
	return core.ModelResult{Text: stubReplies[s.model]}, nil
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

// layout seeds viper with three domains under a temp dir and returns it.
func layout(t *testing.T) string {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	stubMu.Lock()
	stubReplies = map[string]string{}
	stubBuilt = nil
	stubMu.Unlock()

	root := t.TempDir()
	for name, fields := range map[string]map[string]string{
		cmdinternal.DOMAIN_CLASSIFY: {"model": "router", "source": filepath.Join(root, "inbox")},
		cmdinternal.DOMAIN_GENERAL:  {"model": "general-m", "source": filepath.Join(root, "general", "tmp"), "target": filepath.Join(root, "general")},
		cmdinternal.DOMAIN_MEDICAL:  {"model": "medical-m", "source": filepath.Join(root, "medical", "tmp"), "target": filepath.Join(root, "medical")},
	} {
		viper.Set(cmdinternal.DomainKey(name, cmdinternal.FIELD_PROVIDER), "stub")
		for field, value := range fields {
			viper.Set(cmdinternal.DomainKey(name, field), value)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "inbox", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeImage(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{0xff, 0xd8}, 0o600); err != nil {
		t.Fatal(err)
	}
}

func newTestCommand() (*cobra.Command, *bytes.Buffer) {
	var out bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&out)
	c.SetContext(context.Background())
	return c, &out
}

func TestConfigCommandIsAtRoot(t *testing.T) {
	got, _, err := RootCmd.Find([]string{"config"})
	if err != nil {
		t.Fatalf("find config: %v", err)
	}
	if got != gosdkcmd.ConfigCmd {
		t.Fatalf("got %q, want the gosdk config command", got.CommandPath())
	}
}

// classify --apply moves a receipt into the general domain's configured
// source folder, found recursively beneath the classify source.
func TestClassifyRoutesIntoGeneralSource(t *testing.T) {
	root := layout(t)
	stubReplies["router"] = `{"category":"general_receipt"}`
	writeImage(t, filepath.Join(root, "inbox", "nested", "a.jpg"))

	c, out := newTestCommand()
	if err := runClassify(c, classifyOptions{apply: true}); err != nil {
		t.Fatalf("classify: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "general", "tmp", "a.jpg")); err != nil {
		t.Fatalf("image not routed to general source: %v\n%s", err, out)
	}
}

// Each domain builds its own provider from its own block.
func TestProcessBuildsOneProviderPerDomain(t *testing.T) {
	root := layout(t)
	stubReplies["router"] = `{"category":"non_receipt"}`
	for _, dir := range []string{"general/tmp", "medical/tmp"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	c, out := newTestCommand()
	processApply = false
	if err := runProcess(c, nil); err != nil {
		t.Fatalf("process: %v\n%s", err, out)
	}
	want := []string{"router", "general-m", "medical-m"}
	if strings.Join(stubBuilt, ",") != strings.Join(want, ",") {
		t.Fatalf("built providers for %v, want %v", stubBuilt, want)
	}
}

// --provider / --model replace the configured binding for one run only.
func TestOverrideReplacesDomainModel(t *testing.T) {
	layout(t)
	d, err := cmdinternal.LoadDomain(cmdinternal.DOMAIN_GENERAL, cmdinternal.ModelOverride{Model: "bigger"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Provider != "stub" || d.Model != "bigger" {
		t.Fatalf("got %s, want stub:bigger", d)
	}
}

// A route folder inside the classify source would be re-classified on
// every run, so the configuration is refused before any model call.
func TestClassifyRefusesRouteInsideSource(t *testing.T) {
	root := layout(t)
	viper.Set(cmdinternal.DomainKey(cmdinternal.DOMAIN_GENERAL, cmdinternal.FIELD_SOURCE), filepath.Join(root, "inbox", "general"))

	c, _ := newTestCommand()
	err := runClassify(c, classifyOptions{})
	if err == nil || !strings.Contains(err.Error(), "must not lie inside") {
		t.Fatalf("got %v, want a route-inside-source error", err)
	}
}

func TestMissingKeysNameTheConfigKey(t *testing.T) {
	layout(t)
	viper.Set(cmdinternal.DomainKey(cmdinternal.DOMAIN_MEDICAL, cmdinternal.FIELD_TARGET), "")
	viper.Set(cmdinternal.DomainKey(cmdinternal.DOMAIN_CLASSIFY, cmdinternal.FIELD_SOURCE), "")

	if _, err := cmdinternal.LoadDomain(cmdinternal.DOMAIN_CLASSIFY, cmdinternal.ModelOverride{}); err == nil || !strings.Contains(err.Error(), "domain.classify.source") {
		t.Fatalf("got %v, want domain.classify.source error", err)
	}
	c, _ := newTestCommand()
	err := receiptcmd.RunExtract(c, cmdinternal.DOMAIN_MEDICAL, nil, receiptcmd.ExtractOptions{})
	if err == nil || !strings.Contains(err.Error(), "domain.medical.target") {
		t.Fatalf("got %v, want domain.medical.target error", err)
	}
}
