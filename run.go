package prepalert

import (
	"context"
	"log/slog"
	"strings"

	"github.com/mashiike/canyon"
)

type RunOptions struct {
	Mode      string `help:"run mode" env:"PREPALERT_MODE" default:"all" enum:"all,http,worker,webhook"`
	Address   string `help:"run local address" env:"PREPALERT_ADDRESS" default:":8080"`
	Prefix    string `help:"run server prefix" env:"PREPALERT_PREFIX" default:"/"`
	BatchSize int    `help:"run local sqs batch size" env:"PREPALERT_BATCH_SIZE" default:"1"`
}

func (app *App) Run(ctx context.Context, opts *RunOptions) error {
	canyonOpts := []canyon.Option{
		canyon.WithCanyonEnv("PREPALERT_CANYON_"),
		canyon.WithServerAddress(opts.Address, opts.Prefix),
		canyon.WithWorkerBatchSize(opts.BatchSize),
	}

	switch strings.ToLower(opts.Mode) {
	case "http", "webhook":
		slog.InfoContext(ctx, "disable worker", "mode", opts.Mode)
		canyonOpts = append(canyonOpts, canyon.WithDisableWorker())
	case "worker":
		slog.InfoContext(ctx, "disable server", "mode", opts.Mode)
		canyonOpts = append(canyonOpts, canyon.WithDisableServer())
	default:
		// nothing to do
	}
	return canyon.RunWithContext(ctx, app.queueName, app, canyonOpts...)
}
