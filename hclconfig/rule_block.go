package hclconfig

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/queryrunner"
	"github.com/zclconf/go-cty/cty"
)

type RuleBlock struct {
	Name                              string
	Alert                             AlertBlock
	QueriesExpr                       hcl.Expression
	ParamsExpr                        hcl.Expression
	Information                       hcl.Expression
	UpdateAlertMemo                   bool
	PostGraphAnnotation               bool
	MaxGraphAnnotationDescriptionSize *int
	MaxAlertMemoSize                  *int

	Params  cty.Value
	Queries map[string]queryrunner.PreparedQuery
}

func (b *RuleBlock) DecodeBody(body hcl.Body, ctx *hcl.EvalContext, queries queryrunner.PreparedQueries) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "queries",
			},
			{
				Name:     "information",
				Required: true,
			},
			{
				Name: "params",
			},
			{
				Name: "post_graph_annotation",
			},
			{
				Name: "update_alert_memo",
			},
			{
				Name: "max_graph_annotation_description_size",
			},
			{
				Name: "max_alert_memo_size",
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
				query, err := queryrunner.TraversalQuery(variable, queries)
				if err != nil {
					diags = append(diags, &hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "Invalid Relation",
						Detail:   err.Error() + `: rule.queries depends on "query" block, please write as "query.name"`,
						Subject:  variable.SourceRange().Ptr(),
					})
					continue
				}
				b.Queries[query.Name()] = query
			}
		case "information":
			b.Information = attr.Expr
		case "params":
			params, valueDiags := attr.Expr.Value(ctx)
			diags = append(diags, valueDiags...)
			b.Params = params
		case "update_alert_memo":
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.UpdateAlertMemo)...)
			if diags.HasErrors() {
				continue
			}
		case "post_graph_annotation":
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.PostGraphAnnotation)...)
			if diags.HasErrors() {
				continue
			}
		case "max_graph_annotation_description_size":
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.MaxGraphAnnotationDescriptionSize)...)
			if diags.HasErrors() {
				continue
			}
		case "max_alert_memo_size":
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &b.MaxAlertMemoSize)...)
			if diags.HasErrors() {
				continue
			}
		}
	}
	return diags
}

type AlertBlock struct {
	MonitorID   *string
	MonitorName *string
	Any         *bool
	OnOpened    *bool
	OnClosed    *bool
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
				Name: "on_opened",
			},
			{
				Name: "on_closed",
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
		case "on_opened":
			var flag bool
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &flag)...)
			b.OnOpened = &flag
		case "on_closed":
			var flag bool
			diags = append(diags, hclconfig.DecodeExpression(attr.Expr, ctx, &flag)...)
			b.OnClosed = &flag
		}
	}
	return diags
}

type RuleBlocks []*RuleBlock
