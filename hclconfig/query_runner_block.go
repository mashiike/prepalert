package hclconfig

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/prepalert/queryrunner"
)

type QueryRunnerBlock struct {
	Type   string   `hcl:"type,label"`
	Name   string   `hcl:"name,label"`
	Remain hcl.Body `hcl:",remain"`

	Impl queryrunner.QueryRunner
}

func restrictQueryRunnerBlock(body hcl.Body, queryRunnerType string) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "type",
			},
			{
				Name: "namy",
			},
		},
	}
	_, partialBody, diags := body.PartialContent(schema)
	diags = append(diags, queryrunner.RestrictQueryRunnerBlock(queryRunnerType, partialBody)...)
	return diags
}

func (b *QueryRunnerBlock) build(ctx *hcl.EvalContext) hcl.Diagnostics {
	queryRunner, diags := queryrunner.NewQueryRunner(b.Type, b.Name, b.Remain, ctx)
	if diags.HasErrors() {
		return diags
	}
	b.Impl = queryRunner
	return nil
}

type QueryRunnerBlocks []*QueryRunnerBlock

func (blocks QueryRunnerBlocks) Get(queryRunnerType string, name string) (*QueryRunnerBlock, bool) {
	for _, block := range blocks {
		if block.Type != queryRunnerType {
			continue
		}
		if name != block.Name {
			continue
		}
		return block, true
	}
	return nil, false
}

func (blocks QueryRunnerBlocks) build(ctx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, query := range blocks {
		diags = append(diags, query.build(ctx)...)
	}
	return diags
}
