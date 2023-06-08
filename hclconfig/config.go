package hclconfig

import (
	"errors"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/queryrunner"

	"github.com/hashicorp/hcl/v2/ext/dynblock"
	_ "github.com/mashiike/queryrunner/cloudwatchlogsinsights"
	_ "github.com/mashiike/queryrunner/redshiftdata"
	_ "github.com/mashiike/queryrunner/s3select"
)

type Config struct {
	EvalContext *hcl.EvalContext
	Prepalert   PrepalertBlock
	Rules       RuleBlocks
	Queries     queryrunner.PreparedQueries
}

func (cfg *Config) DecodeBody(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
	cfg.EvalContext = ctx.NewChild()
	body = dynblock.Expand(body, ctx)
	queries, body, diags := queryrunner.DecodeBody(body, ctx)
	cfg.Queries = queries
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "prepalert",
			},
			{
				Type:       "rule",
				LabelNames: []string{"name"},
			},
		},
	}
	content, contentDiags := body.Content(schema)
	diags = append(diags, contentDiags...)
	diags = append(diags, hclconfig.RestrictOnlyOneBlock(content, "prepalert")...)
	diags = append(diags, hclconfig.RestrictRequiredBlock(content, "prepalert")...)
	diags = append(diags, hclconfig.RestrictUniqueBlockLabels(content, "rule")...)

	for _, block := range content.Blocks {
		switch block.Type {
		case "prepalert":
			diags = append(diags, cfg.Prepalert.DecodeBody(block.Body, ctx)...)
		case "rule":
			rule := &RuleBlock{
				Name:                block.Labels[0],
				PostGraphAnnotation: true,
				UpdateAlertMemo:     true,
			}
			diags = append(diags, rule.DecodeBody(block.Body, ctx, cfg.Queries)...)
			cfg.Rules = append(cfg.Rules, rule)
		}
	}
	return diags
}

func (cfg *Config) ValidateVersion(version string) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	return cfg.Prepalert.ValidateVersion(version)
}
