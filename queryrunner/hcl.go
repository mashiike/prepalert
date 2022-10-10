package queryrunner

import (
	"fmt"
	"log"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclconfig"
	"github.com/olekukonko/tablewriter"
	"github.com/samber/lo"
	"github.com/zclconf/go-cty/cty"
)

func DecodeBody(body hcl.Body, ctx *hcl.EvalContext) (PreparedQueries, hcl.Body, hcl.Diagnostics) {
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
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
	content, remain, diags := body.PartialContent(schema)
	diags = append(diags, hclconfig.RestrictUniqueBlockLabels(content)...)

	queryRunnerBlocks := make(hcl.Blocks, 0)
	queryBlocks := make(hcl.Blocks, 0)
	for _, block := range content.Blocks {
		switch block.Type {
		case "query_runner":
			queryRunnerBlocks = append(queryRunnerBlocks, block)
		case "query":
			queryBlocks = append(queryBlocks, block)
		}
	}
	runners := make(QueryRunners, 0, len(queryRunnerBlocks))
	for _, block := range queryRunnerBlocks {
		runnerType := block.Labels[0]
		runnerName := block.Labels[1]
		query, buildDiags := NewQueryRunner(runnerType, runnerName, block.Body, ctx)
		diags = append(diags, buildDiags...)
		runners = append(runners, query)
	}
	queries := make(PreparedQueries, 0, len(queryBlocks))
	for _, block := range queryBlocks {
		query, decodeDiags := decodeBodyForQueryBlock(block.Body, ctx, block.Labels[0], runners)
		diags = append(diags, decodeDiags...)
		if decodeDiags.HasErrors() {
			continue
		}
		queries = append(queries, query)
	}

	return queries, remain, diags
}

func decodeBodyForQueryBlock(body hcl.Body, ctx *hcl.EvalContext, name string, queryRunners QueryRunners) (PreparedQuery, hcl.Diagnostics) {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "runner",
				Required: true,
			},
		},
	}
	content, remain, diags := body.PartialContent(schema)
	var queryRunner QueryRunner
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
			traversal := variables[0]
			if traversal.IsRelative() {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `traversal is relative, query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`,
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			if rootName := traversal.RootName(); rootName != "query_runner" {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   fmt.Sprintf(`invalid refarence "%s.*", query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`, rootName),
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			if len(traversal) != 3 {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`,
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			typeAttr, ok := traversal[1].(hcl.TraverseAttr)
			if !ok {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`,
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			nameAttr, ok := traversal[2].(hcl.TraverseAttr)
			if !ok {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   `query.runner depends on "qurey_runner" block,  please write as runner = "query_runner.type.name"`,
					Subject:  traversal.SourceRange().Ptr(),
				})
				continue
			}
			log.Printf("[debug] try runner type `%s` restriction", typeAttr.Name)
			queryRunner, ok = queryRunners.Get(typeAttr.Name, nameAttr.Name)
			if !ok {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid Relation",
					Detail:   fmt.Sprintf(`query_runner "%s.%s" is not found`, typeAttr.Name, nameAttr.Name),
					Subject:  variables[0].SourceRange().Ptr(),
				})
				continue
			}
		}
	}
	if diags.HasErrors() {
		return nil, diags
	}

	preparedQuery, prepareDiags := queryRunner.Prepare(name, remain, ctx)
	diags = append(diags, prepareDiags...)
	return preparedQuery, diags
}

func (qr *QueryResult) MarshalCTYValue() cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"name":  cty.StringVal(qr.Name),
		"query": cty.StringVal(qr.Query),
		"columns": cty.ListVal(lo.Map(qr.Columns, func(column string, _ int) cty.Value {
			return cty.StringVal(column)
		})),
		"rows": cty.ListVal(lo.Map(qr.Rows, func(row []string, _ int) cty.Value {
			return cty.ListVal(lo.Map(row, func(v string, _ int) cty.Value {
				return cty.StringVal(v)
			}))
		})),
		"table": cty.StringVal(qr.ToTable()),
		"markdown_table": cty.StringVal(qr.ToTable(
			func(table *tablewriter.Table) {
				table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
				table.SetCenterSeparator("|")
				table.SetAutoFormatHeaders(false)
				table.SetAutoWrapText(false)
			},
		)),
		"borderless_table": cty.StringVal(qr.ToTable(
			func(table *tablewriter.Table) {
				table.SetCenterSeparator(" ")
				table.SetAutoFormatHeaders(false)
				table.SetAutoWrapText(false)
				table.SetBorder(false)
				table.SetColumnSeparator(" ")
			},
		)),
		"vertical_table": cty.StringVal(qr.ToVertical()),
		"json_lines":     cty.StringVal(qr.ToJSON()),
	})
}
