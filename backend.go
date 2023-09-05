package prepalert

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/ls3viewer"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/zclconf/go-cty/cty"
)

//go:generate mockgen -source=$GOFILE -destination=./mock/mock_$GOFILE -package=mock

type Backend interface {
	http.Handler
	fmt.Stringer
	Upload(ctx context.Context, evalCtx *hcl.EvalContext, name string, body io.Reader) (string, bool, error)
}

type S3Backend struct {
	cfg      *hclconfig.S3BackendBlock
	uploader *manager.Uploader
	h        http.Handler
}

func NewS3Backend(client manager.UploadAPIClient, cfg *hclconfig.S3BackendBlock, authCfg *hclconfig.AuthBlock) (*S3Backend, error) {
	b := &S3Backend{
		cfg:      cfg,
		uploader: manager.NewUploader(client),
	}
	viewerOptFns := []func(*ls3viewer.Options){
		ls3viewer.WithBaseURL(cfg.ViewerBaseURL.String()),
	}
	if !authCfg.IsEmpty() && !cfg.EnableGoogleAuth() {
		viewerOptFns = append(viewerOptFns, ls3viewer.WithBasicAuth(authCfg.ClientID, authCfg.ClientSecret))
	}
	if cfg.EnableGoogleAuth() {
		viewerOptFns = append(viewerOptFns, ls3viewer.WithGoogleOIDC(
			*cfg.ViewerGoogleClientID,
			*cfg.ViewerGoogleClientSecret,
			cfg.ViewerSessionEncryptKey,
			cfg.Allowed,
			cfg.Denied,
		))
	}
	h, err := ls3viewer.New(cfg.BucketName, *cfg.ObjectKeyPrefix, viewerOptFns...)
	if err != nil {
		return nil, fmt.Errorf("initialize ls3viewer:%w", err)
	}
	b.h = h
	return b, nil
}

func (b *S3Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.h.ServeHTTP(w, r)
}

func (b *S3Backend) String() string {
	return fmt.Sprintf("s3_backend{location=s3://%s/%s}", b.cfg.BucketName, *b.cfg.ObjectKeyPrefix)
}

func (b *S3Backend) Upload(ctx context.Context, evalCtx *hcl.EvalContext, name string, body io.Reader) (string, bool, error) {
	expr := *b.cfg.ObjectKeyTemplate
	objectKeyTemplateValue, diags := expr.Value(evalCtx)
	if diags.HasErrors() {
		return "", false, fmt.Errorf("eval object key template: %w", diags)
	}
	if objectKeyTemplateValue.Type() != cty.String {
		return "", false, errors.New("object key template is not string")
	}
	if !objectKeyTemplateValue.IsKnown() {
		return "", false, errors.New("object key template is unknown")
	}
	objectKey := filepath.Join(*b.cfg.ObjectKeyPrefix, objectKeyTemplateValue.AsString(), fmt.Sprintf("%s.txt", name))
	u := b.cfg.ViewerBaseURL.JoinPath(objectKeyTemplateValue.AsString(), fmt.Sprintf("%s.txt", name))
	showDetailsURL := u.String()
	slog.DebugContext(
		ctx,
		"try upload to backend",
		"s3_url", fmt.Sprintf("s3://%s/%s", b.cfg.BucketName, objectKey),
		"show_details_url", showDetailsURL,
	)
	output, err := b.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.cfg.BucketName),
		Key:    aws.String(objectKey),
		Body:   body,
	})
	if err != nil {
		return "", false, fmt.Errorf("upload to backend failed: %w", err)
	}
	slog.InfoContext(ctx, "complete upload to backend", "s3_url", output.Location)
	return showDetailsURL, true, nil
}

type DiscardBackend struct{}

func NewDiscardBackend() *DiscardBackend {
	return &DiscardBackend{}
}

func (b *DiscardBackend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.InfoContext(r.Context(), "backend is not enabled", "status", http.StatusMethodNotAllowed)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func (b *DiscardBackend) String() string {
	return "discard_backend"
}

func (b *DiscardBackend) Upload(ctx context.Context, evalCtx *hcl.EvalContext, name string, body io.Reader) (string, bool, error) {
	return "", false, nil
}
