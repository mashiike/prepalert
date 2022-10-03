package hclconfig

import (
	"fmt"
	"log"

	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/prepalert/internal/generics"
	"github.com/mashiike/prepalert/queryrunner"
)

type RuleBlock struct {
	Name        string
	Alert       AlertBlock
	QueriesExpr hcl.Expression
	ParamsExpr  hcl.Expression
	Infomation  string

	Params  interface{}
	Queries map[string]queryrunner.PreparedQuery
}

func (b *RuleBlock) DecodeBody(body hcl.Body, ctx *hcl.EvalContext, queries queryrunner.PreparedQueries) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "queries",
			},
			{
				Name:     "infomation",
				Required: true,
			},
			{
				Name: "params",
			},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "alert",
			},
		},
	}
	content, diags := body.Content(schema)
	diags = append(diags, hclconfig.RestrictOnlyOneBlock(content, "alert")...)
	var existsAlert bool
	for _, block := range content.Blocks {
		switch block.Type {
		case "alert":
			existsAlert = true
			diags = append(diags, hclconfig.DecodeBody(block.Body, ctx, &b.Alert)...)
		}
	}
	if !existsAlert {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Missing required block",
			Detail:   `The block "alert" is required, but no definition was found. which alerts does this rule respond to?`,
			Subject:  content.MissingItemRange.Ptr(),
		})
	}
	if diags.HasErrors() {
		return diags
	}
	for key, attr := range content.Attributes {
		switch key {
		case "queries":
			variables := attr.Expr.Variables()
			b.Queries = make(map[string]queryrunner.PreparedQuery, len(variables))
			for _, variable := range variables {
				attr, err := GetTraversalAttr(variable, "query", 1)
				if err != nil {
					log.Printf("[debug] get traversal attr failed, expression on %s: %v", variable.SourceRange().String(), err)
					diags = append(diags, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "Invalid Relation",
						Detail:   `rule.queries depends on "query" block, please write as "query.name"`,
						Subject:  variable.SourceRange().Ptr(),
					})
					continue
				}
				query, ok := queries.Get(attr.Name)
				if !ok {
					diags = append(diags, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "Invalid Relation",
						Detail:   fmt.Sprintf("query.%s is not found", attr.Name),
						Subject:  variable.SourceRange().Ptr(),
					})
					continue
				}
				b.Queries[query.Name()] = query
			}
		case "infomation":
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.Infomation)...)
		case "params":
			var params interface{}
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &params)...)
			b.Params = params
		}
	}
	return diags
}

type AlertBlock struct {
	MonitorID   *string
	MonitorName *string
	Any         *bool
	WhenOpened  *bool
	WhenClosed  *bool
}

func (b *AlertBlock) DecodeBody(body hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "monitor_id",
			},
			{
				Name: "monitor_name",
			},
			{
				Name: "any",
			},
			{
				Name: "when_opened",
			},
			{
				Name: "when_closed",
			},
		},
	}
	content, diags := body.Content(schema)
	for key, attr := range content.Attributes {
		switch key {
		case "monitor_id":
			var str string
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &str)...)
			b.MonitorID = &str
		case "monitor_name":
			var str string
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &str)...)
			b.MonitorName = &str
		case "any":
			var any bool
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &any)...)
			b.Any = &any
		case "when_opened":
			var flag bool
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &flag)...)
			b.WhenOpened = &flag
		case "when_closed":
			var flag bool
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &flag)...)
			b.WhenClosed = &flag
		}
	}
	if b.WhenOpened == nil {
		b.WhenOpened = generics.Ptr(false)
	}
	if b.WhenClosed == nil {
		b.WhenClosed = generics.Ptr(true)
	}
	return diags
}

type RuleBlocks []*RuleBlock
