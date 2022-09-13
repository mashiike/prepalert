package hclconfig

import (
	"fmt"
	"log"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

type RuleBlock struct {
	Name        string         `hcl:"name,label"`
	Alert       AlertBlock     `hcl:"alert,block"`
	QueriesExpr hcl.Expression `hcl:"queries"`
	ParamsExpr  hcl.Expression `hcl:"params,optional"`
	Infomation  string         `hcl:"infomation"`

	Params  interface{}
	Queries map[string]*QueryBlock
}

func restrictRuleBlock(body hcl.Body) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "name",
			},
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
	var existsAlert bool
	for _, block := range content.Blocks {
		switch block.Type {
		case "alert":
			existsAlert = true
			buildDiags := restrictAlertBlock(block.Body)
			diags = append(diags, buildDiags...)
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
	return diags
}

func (b *RuleBlock) build(ctx *hcl.EvalContext, queries QueryBlocks) hcl.Diagnostics {
	diags := b.Alert.build(ctx)
	variables := b.QueriesExpr.Variables()
	b.Queries = make(map[string]*QueryBlock, len(variables))
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
		b.Queries[query.Name] = query
	}

	paramsVal, valueDiags := b.ParamsExpr.Value(ctx)
	diags = append(diags, valueDiags...)
	if diags.HasErrors() {
		return diags
	}
	b.Params = convertParams(paramsVal)

	return diags
}

func convertParams(val cty.Value) interface{} {
	if val.IsNull() {
		return nil
	}
	if val.Type().IsObjectType() {
		valueMap := val.AsValueMap()
		params := make(map[string]interface{}, len(valueMap))
		for name, value := range valueMap {
			params[name] = convertParams(value)
		}
		return params
	}
	if val.Type().IsCollectionType() {
		valueSlice := val.AsValueSlice()
		params := make([]interface{}, len(valueSlice))
		for i, value := range valueSlice {
			params[i] = convertParams(value)
		}
		return params
	}
	if strVal, err := convert.Convert(val, cty.String); err == nil {
		return strVal.AsString()
	}
	if numVal, err := convert.Convert(val, cty.Number); err == nil {
		return numVal.AsBigFloat()
	}
	if boolVal, err := convert.Convert(val, cty.Bool); err == nil {
		return boolVal.True()
	}
	return nil
}

func restrictAlertBlock(body hcl.Body) hcl.Diagnostics {
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
		},
	}
	_, diags := body.Content(schema)
	return diags
}

type AlertBlock struct {
	MonitorID   *string `hcl:"monitor_id"`
	MonitorName *string `hcl:"monitor_name"`
	Any         *bool   `hcl:"any"`
}

func (b *AlertBlock) build(ctx *hcl.EvalContext) hcl.Diagnostics {
	return nil
}

type RuleBlocks []*RuleBlock

func (blocks RuleBlocks) build(ctx *hcl.EvalContext, queries QueryBlocks) hcl.Diagnostics {
	var diags hcl.Diagnostics
	for _, rule := range blocks {
		diags = append(diags, rule.build(ctx, queries)...)
	}
	return diags
}
