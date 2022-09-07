package hclconfig

import (
	"errors"
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

type Config struct {
	Prepalert    PrepalertBlock    `hcl:"prepalert,block"`
	Rules        RuleBlocks        `hcl:"rule,block"`
	QueryRunners QueryRunnerBlocks `hcl:"query_runner,block"`
	Queries      QueryBlocks       `hcl:"query,block"`
}

func restrict(body hcl.Body) hcl.Diagnostics {
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
	queryNames := make(map[string]*hcl.Range, 0)
	queryRunnerNames := make(map[string]*hcl.Range, 0)
	for _, block := range content.Blocks {
		switch block.Type {
		case "prepalert":
			restrictDiags := restrictPrepalertBlock(block.Body)
			diags = append(diags, restrictDiags...)
		case "rule":
			restrictDiags := restrictRuleBlock(block.Body)
			diags = append(diags, restrictDiags...)

		case "query_runner":
			if len(block.Labels) != 2 {
				continue
			}
			name := block.Labels[1]
			if r, ok := queryRunnerNames[name]; ok {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  `Duplicate "query_runner" name`,
					Detail:   fmt.Sprintf(`A query runner named "%s" was already declared at %s. Query runner names must unique`, name, r.String()),
					Subject:  block.DefRange.Ptr(),
				})
			}
			queryRunnerNames[name] = block.DefRange.Ptr()
			restrictDiags := restrictQueryRunnerBlock(block.Body, block.Labels[0])
			diags = append(diags, restrictDiags...)
		case "query":
			if len(block.Labels) != 1 {
				continue
			}
			name := block.Labels[0]
			if r, ok := queryNames[name]; ok {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  `Duplicate "query" name`,
					Detail:   fmt.Sprintf(`A query named "%s" was already declared at %s. Query names must unique`, name, r.String()),
					Subject:  block.DefRange.Ptr(),
				})
			}
			queryNames[name] = block.DefRange.Ptr()
			restrictDiags := restrictQueryBlock(block.Body)
			diags = append(diags, restrictDiags...)
		}
	}
	return diags
}

func (cfg *Config) build(ctx *hcl.EvalContext) hcl.Diagnostics {
	diags := cfg.Prepalert.build(ctx)
	if diags.HasErrors() {
		return diags
	}
	diags = append(diags, cfg.QueryRunners.build(ctx)...)
	if diags.HasErrors() {
		return diags
	}
	diags = append(diags, cfg.Queries.build(ctx, cfg.QueryRunners)...)
	if diags.HasErrors() {
		return diags
	}
	diags = append(diags, cfg.Rules.build(ctx, cfg.Queries)...)
	return diags
}

func (cfg *Config) ValidateVersion(version string) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	return cfg.Prepalert.ValidateVersion(version)
}
