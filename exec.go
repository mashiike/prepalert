package prepalert

import (
	"context"
	"errors"
)

type ExecOptions struct {
	AlertID string `arg:"" name:"alert-id" help:"Mackerel AlertID" required:""`
}

func (app *App) Exec(ctx context.Context, opts *ExecOptions) error {
	if !app.WorkerIsReady() {
		return errors.New("worker is not ready, check configureion error")
	}
	body, err := app.mkrSvc.NewEmulatedWebhookBody(ctx, opts.AlertID)
	if err != nil {
		return err
	}
	return app.ExecuteRules(ctx, body)
}
