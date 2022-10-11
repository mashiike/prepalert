package hclconfig

import (
	"fmt"
	"log"
	"net/url"
	"strings"

	gv "github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/prepalert/internal/generics"
)

type PrepalertBlock struct {
	versionConstraints gv.Constraints

	SQSQueueName string
	Service      string
	Auth         *AuthBlock
	S3Backend    *S3BackendBlock
}

func (b *PrepalertBlock) DecodeBody(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "required_version",
			},
			{
				Name:     "sqs_queue_name",
				Required: true,
			},
			{
				Name:     "service",
				Required: true,
			},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "auth",
			},
			{
				Type: "s3_backend",
			},
		},
	}
	content, diags := body.Content(schema)
	diags = append(diags, hclconfig.RestrictOnlyOneBlock(content, "auth", "s3_backend")...)
	if diags.HasErrors() {
		return diags
	}
	for key, attr := range content.Attributes {
		switch key {
		case "required_version":
			var str string
			decodeDiags := hclconfig.DecodeExpression(attr.Expr, ctx, &str)
			if !decodeDiags.HasErrors() && str != "" {
				constraints, err := gv.NewConstraint(str)
				if err != nil {
					decodeDiags = append(decodeDiags, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "Invalid version constraint format",
						Detail:   err.Error(),
						Subject:  attr.Range.Ptr(),
					})
				}
				b.versionConstraints = constraints
			}
			diags = append(diags, decodeDiags...)
		case "sqs_queue_name":
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.SQSQueueName)...)
		case "service":
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.Service)...)
		}
	}
	for _, block := range content.Blocks {
		switch block.Type {
		case "s3_backend":
			var s3Backend S3BackendBlock
			diags = append(diags, hclconfig.DecodeBody(block.Body, ctx, &s3Backend)...)
			b.S3Backend = &s3Backend
		case "auth":
			var auth AuthBlock
			diags = append(diags, hclconfig.DecodeBody(block.Body, ctx, &auth)...)
			b.Auth = &auth
		}
	}
	return diags
}

func (b *PrepalertBlock) ValidateVersion(version string) error {
	if b.versionConstraints == nil {
		log.Println("[warn] required_version is empty. Skip checking required_version.")
		return nil
	}
	versionParts := strings.SplitN(version, "-", 2)
	v, err := gv.NewVersion(versionParts[0])
	if err != nil {
		log.Printf("[warn]: Invalid version format \"%s\". Skip checking required_version.", version)
		// invalid version string (e.g. "current") always allowed
		return nil
	}
	if !b.versionConstraints.Check(v) {
		return fmt.Errorf("version %s does not satisfy constraints required_version: %s", version, b.versionConstraints)
	}
	return nil
}

type AuthBlock struct {
	ClientID     string `hcl:"client_id"`
	ClientSecret string `hcl:"client_secret"`
}

func (b *AuthBlock) IsEmpty() bool {
	if b == nil {
		return true
	}
	return b.ClientID == "" || b.ClientSecret == ""
}

type S3BackendBlock struct {
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

func (b *S3BackendBlock) DecodeBody(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
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
	for key, attr := range content.Attributes {
		switch key {
		case "bucket_name":
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.BucketName)...)
		case "object_key_prefix":
			var str string
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &str)...)
			str = strings.TrimPrefix(str, "/")
			b.ObjectKeyPrefix = &str
		case "object_key_template":
			b.ObjectKeyTemplate = &attr.Expr
		case "viewer_base_url":
			var str string
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &str)...)
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
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &str)...)
			b.ViewerGoogleClientID = &str
		case "viewer_google_client_secret":
			var str string
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &str)...)
			b.ViewerGoogleClientSecret = &str
		case "viewer_session_encrypt_key":
			var str string
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &str)...)
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
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.Allowed)...)
		case "denied":
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.Denied)...)
		}
	}
	if b.ObjectKeyPrefix == nil {
		b.ObjectKeyPrefix = generics.Ptr("prepalert/")
	}
	if b.ObjectKeyTemplate == nil {
		var parseDiags hcl.Diagnostics
		var expr hcl.Expression
		expr, parseDiags = hclsyntax.ParseExpression([]byte(`strftime("%Y/%m/%d/%H/", runtime.event.alert.opened_at)`), "default_object_key_template.hcl", hcl.InitialPos)
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
	return diags
}

func (b *S3BackendBlock) IsEmpty() bool {
	if b == nil {
		return true
	}
	return b.BucketName == "" || b.ObjectKeyPrefix == nil || b.ObjectKeyTemplate == nil || b.ViewerBaseURL == nil
}

func (b *S3BackendBlock) EnableGoogleAuth() bool {
	if b == nil {
		return false
	}
	return b.ViewerSessionEncryptKey != nil && b.ViewerGoogleClientID != nil && b.ViewerGoogleClientSecret != nil
}
