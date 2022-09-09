package hclconfig

import (
	"fmt"
	"log"
	"strings"

	gv "github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
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
	_, diags := body.Content(schema)
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
	BucketName        string  `hcl:"bucket_name"`
	ObjectKeyPrefix   *string `hcl:"object_key_prefix"`
	ObjectKeyTemplate *string `hcl:"object_key_template"`
}

func (b *S3BackendBlock) build(ctx *hcl.EvalContext) hcl.Diagnostics {
	if b.ObjectKeyPrefix == nil {
		b.ObjectKeyPrefix = generics.Ptr("prepalert/")
	} else {
		*b.ObjectKeyPrefix = strings.TrimPrefix(*b.ObjectKeyPrefix, "/")
	}
	if b.ObjectKeyTemplate == nil {
		b.ObjectKeyTemplate = generics.Ptr(*b.ObjectKeyPrefix + "{{ .Alert.OpenedAt | to_time | strftime `%Y/%m/%d/%H` }}/{{ .Alert.ID }}.txt")
	}
	return nil
}
