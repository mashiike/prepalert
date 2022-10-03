package hclconfig

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/prepalert/queryrunner"
)

type QueryRunnerBlock struct {
	Type string
	Name string

	Impl queryrunner.QueryRunner
}

func (b *QueryRunnerBlock) DecodeBody(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
	queryRunner, diags := queryrunner.NewQueryRunner(b.Type, b.Name, body, ctx)
	b.Impl = queryRunner
	return diags
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
