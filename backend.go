package prepalert

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/ls3viewer"
	"github.com/zclconf/go-cty/cty"
)

//go:generate mockgen -source=$GOFILE -destination=./mock/mock_$GOFILE -package=mock

type Backend interface {
	http.Handler
	fmt.Stringer
	Upload(ctx context.Context, evalCtx *hcl.EvalContext, name string, body io.Reader) (string, bool, error)
}

type S3Client interface {
	manager.UploadAPIClient
	ls3viewer.S3Client
}

type S3Backend struct {
	uploader *manager.Uploader
	client   S3Client
	h        http.Handler

	BucketName                    string
	ObjectKeyPrefix               *string
	ObjectKeyTemplate             *hcl.Expression
	ViewerBaseURLString           string
	ViewerGoogleClientID          *string
	ViewerGoogleClientSecret      *string
	ViewerSessionEncryptKeyString *string
	Allowed                       []string
	Denied                        []string

	ViewerBaseURL           *url.URL
	ViewerSessionEncryptKey []byte
}

func (app *App) Backend() Backend {
	return app.backend
}

var GlobalS3Client S3Client

func (app *App) SetupS3Buckend(body hcl.Body) hcl.Diagnostics {
	client := GlobalS3Client
	if client == nil {
		awsCfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "S3 Backend initialization failed",
				Detail:   fmt.Sprintf("can not create aws config: %v", err.Error()),
				Subject:  body.MissingItemRange().Ptr(),
			}}
		}
		client = s3.NewFromConfig(awsCfg)
	}
	b := &S3Backend{
		uploader: manager.NewUploader(client),
		client:   client,
		h:        http.NotFoundHandler(),
	}
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "bucket_name",
				Required: true,
			},
			{
				Name: "object_key_prefix",
			},
			{
				Name: "object_key_template",
			},
			{
				Name:     "viewer_base_url",
				Required: true,
			},
			{
				Name: "viewer_google_client_id",
			},
			{
				Name: "viewer_google_client_secret",
			},
			{
				Name: "viewer_session_encrypt_key",
			},
			{
				Name: "allowed",
			},
			{
				Name: "denied",
			},
		},
	}
	content, diags := body.Content(schema)
	if diags.HasErrors() {
		return diags
	}
	ctx := app.evalCtx.NewChild()
	for key, attr := range content.Attributes {
		switch key {
		case "bucket_name":
			diags = append(diags, gohcl.DecodeExpression(attr.Expr, ctx, &b.BucketName)...)
		case "object_key_prefix":
			var str string
			diags = append(diags, gohcl.DecodeExpression(attr.Expr, ctx, &str)...)
			str = strings.TrimPrefix(str, "/")
			b.ObjectKeyPrefix = &str
		case "object_key_template":
			b.ObjectKeyTemplate = &attr.Expr
		case "viewer_base_url":
			var str string
			diags = append(diags, gohcl.DecodeExpression(attr.Expr, ctx, &str)...)
			if diags.HasErrors() {
				continue
			}
			b.ViewerBaseURLString = str
			u, err := url.Parse(str)
			if err != nil {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid viewer_base_url format",
					Detail:   fmt.Sprintf("can not parse as url : %v", err.Error()),
					Subject:  attr.Range.Ptr(),
				})
				continue
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid viewer_base_url format",
					Detail:   "must scheme http/https",
					Subject:  attr.Range.Ptr(),
				})
				continue
			}
			b.ViewerBaseURL = u
		case "viewer_google_client_id":
			var str string
			diags = append(diags, gohcl.DecodeExpression(attr.Expr, ctx, &str)...)
			b.ViewerGoogleClientID = &str
		case "viewer_google_client_secret":
			var str string
			diags = append(diags, gohcl.DecodeExpression(attr.Expr, ctx, &str)...)
			b.ViewerGoogleClientSecret = &str
		case "viewer_session_encrypt_key":
			var str string
			diags = append(diags, gohcl.DecodeExpression(attr.Expr, ctx, &str)...)
			b.ViewerSessionEncryptKeyString = &str
			b.ViewerSessionEncryptKey = []byte(str)
			keyLen := len(b.ViewerSessionEncryptKey)
			if keyLen != 16 && keyLen != 24 && keyLen != 32 {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid viewer authentication",
					Detail:   "viewer_session_encrypt_key lengths should be 16, 24, or 32",
					Subject:  attr.Range.Ptr(),
				})
				continue
			}
		case "allowed":
			diags = append(diags, gohcl.DecodeExpression(attr.Expr, ctx, &b.Allowed)...)
		case "denied":
			diags = append(diags, gohcl.DecodeExpression(attr.Expr, ctx, &b.Denied)...)
		}
	}
	if b.ViewerBaseURL == nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid viewer_base_url format",
			Detail:   "viewer_base_url is required",
			Subject:  content.MissingItemRange.Ptr(),
		})
	}
	if b.ObjectKeyPrefix == nil {
		b.ObjectKeyPrefix = aws.String("prepalert/")
	}
	if b.ObjectKeyTemplate == nil {
		var parseDiags hcl.Diagnostics
		var expr hcl.Expression
		expr, parseDiags = hclsyntax.ParseExpression([]byte(`strftime("%Y/%m/%d/%H/", event.alert.opened_at)`), "default_object_key_template.hcl", hcl.InitialPos)
		diags = append(diags, parseDiags...)
		b.ObjectKeyTemplate = &expr
	}
	if b.ViewerGoogleClientID != nil || b.ViewerGoogleClientSecret != nil || b.ViewerSessionEncryptKey != nil {
		if b.ViewerGoogleClientID == nil || b.ViewerGoogleClientSecret == nil || b.ViewerSessionEncryptKey == nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid viewer authentication",
				Detail:   "If you want to set Google authentication for a viewer, in that case you need all of viewer_google_client_id, viewer_google_client_secret, and viewer_session_encrypt_key",
				Subject:  content.MissingItemRange.Ptr(),
			})
		}
	}
	if diags.HasErrors() {
		return diags
	}
	viewerOptFns := []func(*ls3viewer.Options){
		func(o *ls3viewer.Options) {
			o.S3Client = client
		},
		ls3viewer.WithBaseURL(b.ViewerBaseURL.String()),
	}
	if !app.EnableBasicAuth() && !b.EnableGoogleAuth() {
		viewerOptFns = append(viewerOptFns, ls3viewer.WithBasicAuth(app.webhookClientID, app.webhookClientSecret))
	}
	if b.EnableGoogleAuth() {
		viewerOptFns = append(viewerOptFns, ls3viewer.WithGoogleOIDC(
			*b.ViewerGoogleClientID,
			*b.ViewerGoogleClientSecret,
			b.ViewerSessionEncryptKey,
			b.Allowed,
			b.Denied,
		))
	}
	h, err := ls3viewer.New(b.BucketName, *b.ObjectKeyPrefix, viewerOptFns...)
	if err != nil {
		return diags.Extend(hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "S3 Backend initialization failed",
			Detail:   fmt.Sprintf("can not create viewer: %v", err.Error()),
			Subject:  content.MissingItemRange.Ptr(),
		}})
	}
	b.h = h
	app.backend = b
	return diags
}

func (b *S3Backend) IsEmpty() bool {
	if b == nil {
		return true
	}
	return b.BucketName == "" || b.ObjectKeyPrefix == nil || b.ObjectKeyTemplate == nil || b.ViewerBaseURL == nil
}

func (b *S3Backend) EnableGoogleAuth() bool {
	if b == nil {
		return false
	}
	return b.ViewerSessionEncryptKey != nil && b.ViewerGoogleClientID != nil && b.ViewerGoogleClientSecret != nil
}

func (b *S3Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b.h.ServeHTTP(w, r)
}

func (b *S3Backend) String() string {
	return fmt.Sprintf("s3_backend{location=s3://%s/%s}", b.BucketName, *b.ObjectKeyPrefix)
}

func (b *S3Backend) Upload(ctx context.Context, evalCtx *hcl.EvalContext, name string, body io.Reader) (string, bool, error) {
	expr := *b.ObjectKeyTemplate
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
	objectKey := filepath.Join(*b.ObjectKeyPrefix, objectKeyTemplateValue.AsString(), fmt.Sprintf("%s.txt", name))
	u := b.ViewerBaseURL.JoinPath(objectKeyTemplateValue.AsString(), fmt.Sprintf("%s.txt", name))
	showDetailsURL := u.String()
	slog.DebugContext(
		ctx,
		"try upload to backend",
		"s3_url", fmt.Sprintf("s3://%s/%s", b.BucketName, objectKey),
		"show_details_url", showDetailsURL,
	)
	output, err := b.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.BucketName),
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
