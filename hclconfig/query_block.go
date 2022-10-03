package hclconfig

import (
	"fmt"
	"log"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/prepalert/queryrunner"
)

type QueryBlock struct {
	Name   string
	Runner queryrunner.QueryRunner
	Impl   queryrunner.PreparedQuery
}

func (b *QueryBlock) DecodeBody(body hcl.Body, ctx *hcl.EvalContext, queryRunners QueryRunnerBlocks) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "runner",
				Required: true,
			},
		},
	}
	content, remain, diags := body.PartialContent(schema)
	for _, attr := range content.Attributes {
		switch attr.Name {
		case "runner":
			variables := attr.Expr.Variables()
			if len(variables) == 0 {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Query Runner",
					Detail:   `can not set constant value. please write as runner = "query_runner.type.name"`,
					Subject:  attr.Expr.Range().Ptr(),
				})
				continue
			}
			if len(variables) != 1 {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Query Runner",
					Detail:   `can not set multiple query runners. please write as runner = "query_runner.type.name"`,
					Subject:  attr.Expr.Range().Ptr(),
				})
				continue
			}
			typeAttr, err := GetTraversalAttr(variables[0], "query_runner", 1)
			if err != nil {
				log.Printf("[debug] get traversal attr failed, expression on %s: %v", variables[0].SourceRange().String(), err)
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `query.runner depends on "qurey_runner" block, please write as runner = "query_runner.type.name"`,
					Subject:  variables[0].SourceRange().Ptr(),
				})
				continue
			}
			nameAttr, err := GetTraversalAttr(variables[0], "query_runner", 2)
			if err != nil {
				log.Printf("[debug] get traversal attr failed, expression on %s: %v", variables[0].SourceRange().String(), err)
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `query.runner depends on "qurey_runner" block, please write as runner = "query_runner.type.name"`,
					Subject:  variables[0].SourceRange().Ptr(),
				})
				continue
			}
			log.Printf("[debug] try runner type `%s` restriction", typeAttr.Name)
			queryRunner, ok := queryRunners.Get(typeAttr.Name, nameAttr.Name)
			if !ok {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   fmt.Sprintf("query_runner `%s.%s` is not found", typeAttr.Name, nameAttr.Name),
					Subject:  variables[0].SourceRange().Ptr(),
				})
				continue
			}
			b.Runner = queryRunner.Impl
		}
	}
	if diags.HasErrors() {
		return diags
	}

	preparedQuery, prepareDiags := b.Runner.Prepare(b.Name, remain, ctx)
	diags = append(diags, prepareDiags...)
	b.Impl = preparedQuery
	return diags
}

type QueryBlocks []*QueryBlock

func (blocks QueryBlocks) Get(name string) (*QueryBlock, bool) {
	for _, block := range blocks {
		if name == block.Name {
			return block, true
		}
	}
	return nil, false
}
