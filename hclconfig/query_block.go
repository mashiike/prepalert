package hclconfig

import (
	"fmt"
	"log"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/prepalert/queryrunner"
)

type QueryBlock struct {
	Name       string         `hcl:"name,label"`
	RunnerExpr hcl.Expression `hcl:"runner"`
	Remain     hcl.Body       `hcl:",remain"`

	Impl queryrunner.PreparedQuery
}

func restrictQueryBlock(body hcl.Body) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "name",
			},
			{
				Name:     "runner",
				Required: true,
			},
		},
	}
	content, partialBody, diags := body.PartialContent(schema)
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
			diags = append(diags, queryrunner.RestrictQueryBlock(typeAttr.Name, partialBody)...)
		}
	}
	return diags
}

func (b *QueryBlock) build(ctx *hcl.EvalContext, queryRunners QueryRunnerBlocks) hcl.Diagnostics {
	var diags hcl.Diagnostics
	variables := b.RunnerExpr.Variables()
	attr, err := GetTraversalAttr(variables[0], "query_runner", 2)
	if err != nil {
		log.Printf("[debug] get traversal attr failed, expression on %s: %v", variables[0].SourceRange().String(), err)
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid Relation",
			Detail:   `query.runner depends on "qurey_runner" block, please write as runner = "query_runner.type.name"`,
			Subject:  variables[0].SourceRange().Ptr(),
		})
		return diags
	}
	queryRunner, ok := queryRunners.Get(attr.Name)
	if !ok {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid Relation",
			Detail:   fmt.Sprintf("query_runner `%s` is not found", attr.Name),
			Subject:  variables[0].SourceRange().Ptr(),
		})
		return diags
	}
	preparedQuery, prepareDiags := queryRunner.Impl.Prepare(b.Name, b.Remain, ctx)
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

func (blocks QueryBlocks) build(ctx *hcl.EvalContext, queryRunners QueryRunnerBlocks) hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, query := range blocks {
		diags = append(diags, query.build(ctx, queryRunners)...)
	}
	return diags
}
