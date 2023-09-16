package prepalert

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/dynblock"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mashiike/hclutil"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type LoadConfigOptions struct {
	DiagnosticDestination io.Writer
	Color                 *bool
	Width                 *uint
}

func (app *App) LoadConfig(dir string, optFns ...func(*LoadConfigOptions)) error {
	opt := &LoadConfigOptions{}
	for _, optFn := range optFns {
		optFn(opt)
	}
	body, writer, diags := hclutil.Parse(dir)
	if diags.HasErrors() {
		return writer.WriteDiagnostics(diags)
	}
	if opt.DiagnosticDestination != nil {
		writer.SetOutput(opt.DiagnosticDestination)
	}
	if opt.Color != nil {
		writer.SetColor(*opt.Color)
	}
	if opt.Width != nil {
		writer.SetWidth(*opt.Width)
	}
	app.diagWriter = writer
	evalCtx := hclutil.NewEvalContext(
		hclutil.WithFilePath(dir),
	)
	diags = append(diags, app.DecodeBody(body, evalCtx)...)
	return writer.WriteDiagnostics(diags)
}

func (app *App) DecodeBody(body hcl.Body, evalCtx *hcl.EvalContext) hcl.Diagnostics {
	evalCtx = app.WithPrepalertFunctions(evalCtx)
	evalCtx = hclutil.WithValue(evalCtx, "var.version", cty.StringVal(Version))
	body, evalCtx, diags := hclutil.DecodeLocals(body, evalCtx)
	app.evalCtx = evalCtx
	body = dynblock.Expand(body, evalCtx)
	if diags.HasErrors() {
		return diags
	}
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "prepalert",
			},
			{
				Type:       "provider",
				LabelNames: []string{"type"},
			},
			{
				Type:       "query",
				LabelNames: []string{"type", "name"},
			},
			{
				Type:       "rule",
				LabelNames: []string{"name"},
			},
		},
	}
	content, contentDiags := body.Content(schema)
	diags = diags.Extend(contentDiags)
	diags = diags.Extend(hclutil.RestrictBlock(content, []hclutil.BlockRestrictionSchema{
		{
			Type:     "prepalert",
			Required: true,
			Unique:   true,
		},
		{
			Type:         "query",
			UniqueLabels: true,
		},
		{
			Type:         "rule",
			UniqueLabels: true,
		},
	}...))
	if diags.HasErrors() {
		return diags
	}
	blocksByType := content.Blocks.ByType()
	diags = diags.Extend(app.decodePrepalertBlock(blocksByType["prepalert"][0].Body))
	diags = diags.Extend(app.decodeProviderBlocks(blocksByType["provider"]))
	diags = diags.Extend(app.decodeQueryBlocks(blocksByType["query"]))
	diags = diags.Extend(app.decodeRuleBlocks(blocksByType["rule"]))
	return diags
}

func (app *App) decodePrepalertBlock(body hcl.Body) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "required_version",
			},
			{
				Name:     "sqs_queue_name",
				Required: true,
			},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "auth",
			},
			{
				Type: "s3_backend",
			},
		},
	}
	content, diags := body.Content(schema)
	if diags.HasErrors() {
		return diags
	}
	diags.Extend(hclutil.RestrictBlock(content, []hclutil.BlockRestrictionSchema{
		{
			Type:   "auth",
			Unique: true,
		},
		{
			Type:   "s3_backend",
			Unique: true,
		},
	}...))
	for name, attr := range content.Attributes {
		switch name {
		case "required_version":
			var rv hclutil.VersionConstraints
			diags = diags.Extend(rv.DecodeExpression(attr.Expr, app.evalCtx))
			if Version == "current" {
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagWarning,
					Summary:  `required_version validation`,
					Detail:   `required_version is not validated because version is "current"`,
					Subject:  attr.Expr.Range().Ptr(),
				})
				continue
			}
			if err := rv.ValidateVersion(Version); err != nil {
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  `required_version validation`,
					Detail:   err.Error(),
					Subject:  attr.Expr.Range().Ptr(),
				})
			}
		case "sqs_queue_name":
			diags = diags.Extend(gohcl.DecodeExpression(attr.Expr, app.evalCtx, &app.queueName))
		}
	}
	if diags.HasErrors() {
		return diags
	}
	if blocks := content.Blocks.OfType("auth"); len(blocks) > 0 {
		attr, attrDiags := blocks[0].Body.JustAttributes()
		diags = diags.Extend(attrDiags)
		if !attrDiags.HasErrors() {
			for name, attr := range attr {
				switch name {
				case "client_id":
					diags = diags.Extend(gohcl.DecodeExpression(attr.Expr, app.evalCtx, &app.webhookClientID))
				case "client_secret":
					diags = diags.Extend(gohcl.DecodeExpression(attr.Expr, app.evalCtx, &app.webhookClientSecret))
				default:
					diags = diags.Append(&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  `auth attribute validation`,
						Detail:   fmt.Sprintf("attribute %q is not supported", name),
						Subject:  attr.NameRange.Ptr(),
					})
				}
			}
		}
	}
	if blocks := content.Blocks.OfType("s3_backend"); len(blocks) > 0 {
		diags = diags.Extend(app.SetupS3Buckend(blocks[0].Body))
	}
	return diags
}

func (app *App) decodeProviderBlocks(blocks hcl.Blocks) hcl.Diagnostics {
	app.providers = make(map[string]Provider, 0)
	app.providerParameters = make(ProviderParameters, 0)
	var diags hcl.Diagnostics
	for _, block := range blocks {
		switch block.Labels[0] {
		case "prepalert", "provider", "query", "rule":
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  `Provider creation failed`,
				Detail:   fmt.Sprintf("provider name %q is reserved", block.Labels[0]),
				Subject:  block.TypeRange.Ptr(),
			})
			continue
		default:
		}
		pp := &ProviderParameter{
			Type:   block.Labels[0],
			Name:   "default",
			params: make(map[string]cty.Value),
		}
		attrs, attrDiags := block.Body.JustAttributes()
		if attrDiags.HasErrors() {
			diags = diags.Extend(attrDiags)
			continue
		}
		for name, attr := range attrs {
			switch name {
			case "ailias":
				diags = diags.Extend(gohcl.DecodeExpression(attr.Expr, app.evalCtx, &pp.Name))
			default:
				value, valueDiags := attr.Expr.Value(app.evalCtx)
				diags = diags.Extend(valueDiags)
				pp.params[name] = value
			}
		}
		var jsonParams json.RawMessage
		if err := hclutil.UnmarshalCTYValue(cty.ObjectVal(pp.params), &jsonParams); err != nil {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  `Provider creation failed`,
				Detail:   err.Error(),
				Subject:  block.TypeRange.Ptr(),
			})
			continue
		}
		pp.Params = jsonParams
		provider, err := NewProvider(pp)
		if err != nil {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  `Provider creation failed`,
				Detail:   err.Error(),
				Subject:  block.TypeRange.Ptr(),
			})
			continue
		}
		app.providerParameters = append(app.providerParameters, pp)
		app.providers[pp.String()] = provider
	}
	val, err := app.providerParameters.MarshalCTYValue()
	if err != nil {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  `Provider creation failed`,
			Detail:   err.Error(),
			Subject:  blocks[0].TypeRange.Ptr(),
		})
		return diags
	}
	app.evalCtx = hclutil.WithVariables(app.evalCtx, val.AsValueMap())
	return diags
}

func (app *App) decodeQueryBlocks(blocks hcl.Blocks) hcl.Diagnostics {
	app.queries = make(map[string]Query, 0)
	var diags hcl.Diagnostics
	commonQuerySchema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "provider",
			},
		},
	}
	for _, block := range blocks {
		content, remain, contentDiags := block.Body.PartialContent(commonQuerySchema)
		diags = diags.Extend(contentDiags)
		if contentDiags.HasErrors() {
			continue
		}
		fqn := block.Labels[0] + ".default"
		for name, attr := range content.Attributes {
			switch name {
			case "provider":
				variables := attr.Expr.Variables()
				if len(variables) != 1 {
					diags = diags.Append(&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  `Query creation failed`,
						Detail:   "multiple provider not supported",
						Subject:  attr.NameRange.Ptr(),
					})
					continue
				}
				fqn = hclutil.TraversalToString(variables[0])
			default:
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  `Query creation failed`,
					Detail:   fmt.Sprintf("attribute %q is not supported", name),
					Subject:  attr.NameRange.Ptr(),
				})
				continue
			}
		}
		provider, ok := app.providers[fqn]
		if !ok {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  `Query creation failed`,
				Detail:   fmt.Sprintf("provider %q not found", fqn),
				Subject:  block.TypeRange.Ptr(),
			})
			continue
		}
		query, err := provider.NewQuery(block.Labels[1], remain, app.evalCtx)
		if err != nil {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  `Query creation failed`,
				Detail:   err.Error(),
				Subject:  block.TypeRange.Ptr(),
			})
			continue
		}
		queryFQN := "query." + block.Labels[0] + "." + block.Labels[1]
		app.queries[queryFQN] = query
	}
	return diags
}

func (app *App) decodeRuleBlocks(blocks hcl.Blocks) hcl.Diagnostics {
	var diags hcl.Diagnostics
	mkrSvcFunc := func() *MackerelService {
		return app.mkrSvc
	}
	for _, block := range blocks {
		rule := NewRule(mkrSvcFunc, app.backend, block.Labels[0])
		diags = diags.Extend(rule.DecodeBody(block.Body, app.evalCtx))
		app.rules = append(app.rules, rule)
	}
	return diags
}

func (app *App) NewEvalContext(body *WebhookBody) (*hcl.EvalContext, error) {
	if app.evalCtx == nil {
		app.evalCtx = hclutil.NewEvalContext()
	}
	webhook, err := hclutil.MarshalCTYValue(body)
	if err != nil {
		return app.evalCtx.NewChild(), fmt.Errorf("failed marshal Mackerel webhook body to cty value: %w", err)
	}
	return hclutil.WithValue(app.evalCtx, webhookHCLPrefix, webhook), nil
}

func newConvertFunctionForQueryResult(
	description string,
	f func(qr *QueryResult) (string, error),
) function.Function {
	return function.New(&function.Spec{
		Description: description,
		Params: []function.Parameter{
			{
				Name: "query",
				Type: cty.Object(map[string]cty.Type{
					"status": cty.String,
					"result": cty.Object(map[string]cty.Type{
						"name":    cty.String,
						"query":   cty.String,
						"columns": cty.List(cty.String),
						"rows":    cty.List(cty.List(cty.DynamicPseudoType)),
					}),
				}),
			},
		},
		VarParam: nil,
		Type:     function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			var query struct {
				FQN    string       `cty:"fqn"`
				Status string       `cty:"status"`
				Error  string       `cty:"error"`
				Result *QueryResult `cty:"result"`
			}
			if err := hclutil.UnmarshalCTYValue(args[0], &query); err != nil {
				return cty.UnknownVal(cty.String), fmt.Errorf("failed unmarshal query: %w", err)
			}
			switch query.Status {
			case "success":
				str, err := f(query.Result)
				if err != nil {
					return cty.UnknownVal(cty.String), fmt.Errorf("failed convert query result: %w", err)
				}
				return cty.StringVal(str), nil
			case "failed":
				return cty.StringVal(fmt.Sprintf("[query %q failed: %s]", query.FQN, query.Error)), nil
			case "running":
				return cty.StringVal(fmt.Sprintf("[query %q running]", query.FQN)), nil
			}
			return cty.UnknownVal(cty.String), errors.New("query.status unknown")
		},
	})
}

func (app *App) WithPrepalertFunctions(evalCtx *hcl.EvalContext) *hcl.EvalContext {
	child := evalCtx.NewChild()
	child.Functions = map[string]function.Function{
		"result_to_table": newConvertFunctionForQueryResult(
			"convert query_result to table format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToTable(), nil
			},
		),
		"result_to_jsonlines": newConvertFunctionForQueryResult(
			"convert query_result to jsonlines format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToJSONLines(), nil
			},
		),
		"result_to_vertical": newConvertFunctionForQueryResult(
			"convert query_result to vertical table format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToVertical(), nil
			},
		),
		"result_to_markdown": newConvertFunctionForQueryResult(
			"convert query_result to markdown table format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToMarkdownTable(), nil
			},
		),
		"result_to_borderless": newConvertFunctionForQueryResult(
			"convert query_result to borderless table format function",
			func(qr *QueryResult) (string, error) {
				return qr.ToBorderlessTable(), nil
			},
		),
		"has_prefix": function.New(&function.Spec{
			Params: []function.Parameter{
				{
					Name: "s",
					Type: cty.String,
				},
				{
					Name: "prefix",
					Type: cty.String,
				},
			},
			Type: function.StaticReturnType(cty.Bool),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				if !args[0].IsKnown() || !args[1].IsKnown() {
					return cty.UnknownVal(cty.Bool), errors.New("args is unknown")
				}
				if args[0].IsNull() || args[1].IsNull() {
					return cty.BoolVal(false), nil
				}
				if args[0].Type() != cty.String || args[1].Type() != cty.String {
					return cty.BoolVal(false), errors.New("args is not string")
				}
				return cty.BoolVal(strings.HasPrefix(args[0].AsString(), args[1].AsString())), nil
			},
		}),
		"has_suffix": function.New(&function.Spec{
			Params: []function.Parameter{
				{
					Name: "s",
					Type: cty.String,
				},
				{
					Name: "suffix",
					Type: cty.String,
				},
			},
			Type: function.StaticReturnType(cty.Bool),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				if !args[0].IsKnown() || !args[1].IsKnown() {
					return cty.UnknownVal(cty.Bool), errors.New("args is unknown")
				}
				if args[0].IsNull() || args[1].IsNull() {
					return cty.BoolVal(false), nil
				}
				if args[0].Type() != cty.String || args[1].Type() != cty.String {
					return cty.BoolVal(false), errors.New("args is not string")
				}
				return cty.BoolVal(strings.HasSuffix(args[0].AsString(), args[1].AsString())), nil
			},
		}),
		"get_monitor": function.New(&function.Spec{
			Params: []function.Parameter{
				{
					Name: "alert",
					Type: cty.Object(map[string]cty.Type{
						"trigger": cty.String,
						"id":      cty.String,
					}),
				},
			},
			Type: function.StaticReturnType(cty.Object(map[string]cty.Type{
				"id":   cty.String,
				"name": cty.String,
				"type": cty.String,
			})),
			Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
				var alert Alert
				if err := hclutil.UnmarshalCTYValue(args[0], &alert); err != nil {
					return cty.UnknownVal(cty.Object(map[string]cty.Type{
						"id":   cty.String,
						"name": cty.String,
						"type": cty.String,
					})), fmt.Errorf("failed unmarshal alert: %w", err)
				}
				if alert.Trigger != "monitor" {
					return cty.ObjectVal(map[string]cty.Value{
						"id":   cty.NullVal(cty.String),
						"name": cty.NullVal(cty.String),
						"type": cty.NullVal(cty.String),
					}), nil
				}
				a, err := app.mkrSvc.client.GetAlert(alert.ID)
				if err != nil {
					return cty.UnknownVal(cty.Object(map[string]cty.Type{
						"id":   cty.String,
						"name": cty.String,
						"type": cty.String,
					})), fmt.Errorf("failed get alert: %w", err)
				}
				m, err := app.mkrSvc.client.GetMonitor(a.MonitorID)
				if err != nil {
					return cty.UnknownVal(cty.Object(map[string]cty.Type{
						"id":   cty.String,
						"name": cty.String,
						"type": cty.String,
					})), fmt.Errorf("failed get monitor: %w", err)
				}
				return cty.ObjectVal(map[string]cty.Value{
					"id":   cty.StringVal(m.MonitorID()),
					"name": cty.StringVal(m.MonitorName()),
					"type": cty.StringVal(m.MonitorType()),
				}), nil
			},
		}),
	}
	return child
}
