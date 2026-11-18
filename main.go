// Command ocr_agent classifies document images with a vision model and
// archives them per domain. Commands live in cmd/, domain logic in svc/.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bizshuk/gosdk/cmd/config"
	gosdkconfig "github.com/bizshuk/gosdk/config"
	"github.com/bizshuk/ocr_agent/cmd"
)

// defaultSettings is what `ocr_agent config default` writes into
// ~/.config/ocr_agent/settings.json.
//
//go:embed config/example/settings.example.json
var defaultSettings []byte

func main() {
	gosdkconfig.Default(gosdkconfig.WithAppName(cmd.APP_NAME))
	config.MustRegisterDefault("settings.json", defaultSettings)

	// Ctrl-C and SIGTERM cancel the run's context, so an in-flight model
	// request is abandoned instead of stranding a half-processed batch.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmd.APP_NAME, err)
		os.Exit(1)
	}
}
