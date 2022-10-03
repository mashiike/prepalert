package hclconfig

import (
	"errors"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclconfig"
)

type Config struct {
	Prepalert    PrepalertBlock
	Rules        RuleBlocks
	QueryRunners QueryRunnerBlocks
	Queries      QueryBlocks
}

func (cfg *Config) DecodeBody(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "prepalert",
			},
			{
				Type:       "rule",
				LabelNames: []string{"name"},
			},
			{
				Type:       "query_runner",
				LabelNames: []string{"type", "name"},
			},
			{
				Type:       "query",
				LabelNames: []string{"name"},
			},
		},
	}
	content, diags := body.Content(schema)
	diags = append(diags, hclconfig.RestrictOnlyOneBlock(content, "prepalert")...)
	diags = append(diags, hclconfig.RestrictUniqueBlockLabels(content)...)

	queryRunnerBlocks := make(hcl.Blocks, 0)
	queryBlocks := make(hcl.Blocks, 0)
	ruleBlocks := make(hcl.Blocks, 0)
	for _, block := range content.Blocks {
		switch block.Type {
		case "prepalert":
			diags = append(diags, cfg.Prepalert.DecodeBody(block.Body, ctx)...)
		case "query_runner":
			queryRunnerBlocks = append(queryRunnerBlocks, block)
		case "query":
			queryBlocks = append(queryBlocks, block)
		case "rule":
			ruleBlocks = append(ruleBlocks, block)
		}
	}
	for _, block := range queryRunnerBlocks {
		runner := &QueryRunnerBlock{
			Type: block.Labels[0],
			Name: block.Labels[1],
		}
		diags = append(diags, hclconfig.DecodeBody(block.Body, ctx, runner)...)
		cfg.QueryRunners = append(cfg.QueryRunners, runner)
	}
	for _, block := range queryBlocks {
		query := &QueryBlock{
			Name: block.Labels[0],
		}
		diags = append(diags, query.DecodeBody(block.Body, ctx, cfg.QueryRunners)...)
		cfg.Queries = append(cfg.Queries, query)
	}
	for _, block := range ruleBlocks {
		rule := &RuleBlock{
			Name: block.Labels[0],
		}
		diags = append(diags, rule.DecodeBody(block.Body, ctx, cfg.Queries)...)
		cfg.Rules = append(cfg.Rules, rule)
	}
	return diags
}

func (cfg *Config) ValidateVersion(version string) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	return cfg.Prepalert.ValidateVersion(version)
}
