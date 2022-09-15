package hclconfig

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"text/template"

	gv "github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/prepalert/internal/funcs"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

type PrepalertBlock struct {
	RequiredVersionExpr hcl.Expression `hcl:"required_version"`
	versionConstraints  gv.Constraints

	SQSQueueName string          `hcl:"sqs_queue_name"`
	Service      string          `hcl:"service"`
	Auth         *AuthBlock      `hcl:"auth,block"`
	S3Backend    *S3BackendBlock `hcl:"s3_backend,block"`
}

func restrictPrepalertBlock(body hcl.Body) hcl.Diagnostics {
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
	for _, block := range content.Blocks {
		if block.Type == "s3_backend" {
			diags = append(diags, restrictS3BackendBlock(block.Body)...)
		}
	}
	return diags
}

func (b *PrepalertBlock) build(ctx *hcl.EvalContext) hcl.Diagnostics {
	value, diags := b.RequiredVersionExpr.Value(ctx)
	if diags.HasErrors() {
		return diags
	}
	var err error
	value, err = convert.Convert(value, cty.String)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid required_version format",
			Detail:   fmt.Sprintf("can not decode as string: %v", err.Error()),
			Subject:  b.RequiredVersionExpr.Range().Ptr(),
		})
		return diags
	}

	if value.IsNull() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid required_version format",
			Detail:   "required_version is nil",
			Subject:  b.RequiredVersionExpr.Range().Ptr(),
		})
		return diags
	}

	if !value.IsKnown() {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid required_version format",
			Detail:   "value is not known",
			Subject:  b.RequiredVersionExpr.Range().Ptr(),
		})
		return diags
	}
	constraints, err := gv.NewConstraint(value.AsString())
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid version constraint format",
			Detail:   err.Error(),
			Subject:  b.RequiredVersionExpr.Range().Ptr(),
		})
		return diags
	}
	b.versionConstraints = constraints

	if b.S3Backend != nil {
		diags = append(diags, b.S3Backend.build(ctx)...)
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
	BucketName                    string  `hcl:"bucket_name"`
	ObjectKeyPrefix               *string `hcl:"object_key_prefix"`
	ObjectKeyTemplateString       *string `hcl:"object_key_template"`
	ViewerBaseURLString           string  `hcl:"viewer_base_url"`
	ViewerGoogleClientID          *string `hcl:"viewer_google_client_id"`
	ViewerGoogleClientSecret      *string `hcl:"viewer_google_client_secret"`
	ViewerSessionEncryptKeyString *string `hcl:"viewer_session_encrypt_key"`

	ObjectKeyTemplate       *template.Template
	ViewerBaseURL           *url.URL
	Remain                  hcl.Body `hcl:",remain"`
	ViewerSessionEncryptKey []byte
}

func restrictS3BackendBlock(body hcl.Body) hcl.Diagnostics {
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
				Name: "viewer_base_url",
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
		},
	}
	_, diags := body.Content(schema)
	return diags
}

func (b *S3BackendBlock) build(ctx *hcl.EvalContext) hcl.Diagnostics {
	if b.ObjectKeyPrefix == nil {
		b.ObjectKeyPrefix = generics.Ptr("prepalert/")
	} else {
		*b.ObjectKeyPrefix = strings.TrimPrefix(*b.ObjectKeyPrefix, "/")
	}
	if b.ObjectKeyTemplateString == nil {
		b.ObjectKeyTemplateString = generics.Ptr("{{ .Alert.OpenedAt | to_time | strftime `%Y/%m/%d/%H` }}/")
	}
	tmpl, err := template.New("object_key_template").Funcs(funcs.QueryTemplateFuncMap).Parse(*b.ObjectKeyTemplateString)
	var diags hcl.Diagnostics
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid object_key_template format",
			Detail:   fmt.Sprintf("can not parse as go template : %v", err.Error()),
			Subject:  b.Remain.MissingItemRange().Ptr(),
		})
	}
	b.ObjectKeyTemplate = tmpl
	u, err := url.Parse(b.ViewerBaseURLString)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid viewer_base_url format",
			Detail:   fmt.Sprintf("can not parse as url : %v", err.Error()),
			Subject:  b.Remain.MissingItemRange().Ptr(),
		})
		return diags
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid viewer_base_url format",
			Detail:   "must scheme http/https",
			Subject:  b.Remain.MissingItemRange().Ptr(),
		})
		return diags
	}
	b.ViewerBaseURL = u
	if b.ViewerGoogleClientID != nil || b.ViewerGoogleClientSecret != nil || b.ViewerSessionEncryptKeyString != nil {
		if b.ViewerGoogleClientID == nil || b.ViewerGoogleClientSecret == nil || b.ViewerSessionEncryptKeyString == nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid viewer authentication",
				Detail:   "If you want to set Google authentication for a viewer, in that case you need all of viewer_google_client_id, viewer_google_client_secret, and viewer_session_encrypt_key",
				Subject:  b.Remain.MissingItemRange().Ptr(),
			})
			return diags
		}
	}
	if b.ViewerSessionEncryptKeyString != nil {
		b.ViewerSessionEncryptKey = []byte(*b.ViewerSessionEncryptKeyString)
		keyLen := len(b.ViewerSessionEncryptKey)
		if keyLen != 16 && keyLen != 24 && keyLen != 32 {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid viewer authentication",
				Detail:   "viewer_session_encrypt_key lengths should be 16, 24, or 32",
				Subject:  b.Remain.MissingItemRange().Ptr(),
			})
			return diags
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
