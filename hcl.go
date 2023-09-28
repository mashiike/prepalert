package prepalert

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/dynblock"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/hclutil"
	"github.com/mashiike/prepalert/plugin"
	"github.com/mashiike/prepalert/provider"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type LoadConfigOptions struct {
	DiagnosticDestination io.Writer
	Color                 *bool
	Width                 *uint
}

func (app *App) LoadConfig(dir string, optFns ...func(*LoadConfigOptions)) error {
	app.loadingConfig = true
	defer func() {
		app.loadingConfig = false
	}()
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
	if !diags.HasErrors() {
		slog.Debug("set worker is prepared")
		app.workerPrepared = true
	}
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
		},
	}
	content, remain, contentDiags := body.PartialContent(schema)
	diags = diags.Extend(contentDiags)
	diags = diags.Extend(hclutil.RestrictBlock(content, []hclutil.BlockRestrictionSchema{
		{
			Type:     "prepalert",
			Required: true,
			Unique:   true,
		},
	}...))
	if diags.HasErrors() {
		return diags
	}
	blocksByType := content.Blocks.ByType()
	diags = diags.Extend(app.decodePrepalertBlock(blocksByType["prepalert"][0].Body))
	schema = &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
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
	content, contentDiags = remain.Content(schema)
	diags = diags.Extend(contentDiags)
	diags = diags.Extend(hclutil.RestrictBlock(content, []hclutil.BlockRestrictionSchema{
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
	blocksByType = content.Blocks.ByType()
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
				Type: "plugins",
			},
			{
				Type: "auth",
			},
			{
				Type:       "backend",
				LabelNames: []string{"type"},
			},
		},
	}
	content, diags := body.Content(schema)
	if diags.HasErrors() {
		return diags
	}
	diags.Extend(hclutil.RestrictBlock(content, []hclutil.BlockRestrictionSchema{
		{
			Type:   "plugins",
			Unique: true,
		},
		{
			Type:   "auth",
			Unique: true,
		},
		{
			Type:   "backend",
			Unique: true, //TODO Multiple backend, none unique
		},
	}...))
	for name, attr := range content.Attributes {
		switch name {
		case "required_version":
			var rv hclutil.VersionConstraints
			diags = diags.Extend(rv.DecodeExpression(attr.Expr, app.evalCtx))
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
			app.webhookServerPrepared = true
		}
	}
	if diags.HasErrors() {
		return diags
	}
	if blocks := content.Blocks.OfType("plugins"); len(blocks) > 0 {
		attrs, attrDiags := blocks[0].Body.JustAttributes()
		diags = diags.Extend(attrDiags)
		if !attrDiags.HasErrors() {
			for name, attr := range attrs {
				value, valueDiags := attr.Expr.Value(app.evalCtx)
				diags = diags.Extend(valueDiags)
				if valueDiags.HasErrors() {
					continue
				}
				var loadPluginConfig LoadPluginConfig
				if err := hclutil.UnmarshalCTYValue(value, &loadPluginConfig); err != nil {
					diags = diags.Append(&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  `plugin attribute validation`,
						Detail:   err.Error(),
						Subject:  attr.NameRange.Ptr(),
					})
					continue
				}
				loadPluginConfig.PluginName = name
				if err := app.LoadPlugin(context.Background(), &loadPluginConfig); err != nil {
					diags = diags.Append(&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  `plugin attribute validation`,
						Detail:   err.Error(),
						Subject:  attr.NameRange.Ptr(),
					})
					continue
				}
			}
		}
	}
	if blocks := content.Blocks.OfType("auth"); len(blocks) > 0 {
		attr, attrDiags := blocks[0].Body.JustAttributes()
		diags = diags.Extend(attrDiags)
		if attrDiags.HasErrors() {
			app.webhookServerPrepared = false
		} else {
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
			if diags.HasErrors() {
				app.webhookServerPrepared = false
			}
		}
	}
	if blocks := content.Blocks.OfType("backend"); len(blocks) > 0 {
		for _, block := range blocks {
			switch block.Labels[0] {
			case "s3":
				diags = diags.Extend(app.SetupS3Buckend(blocks[0].Body))
			default:
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  `backend block validation`,
					Detail:   fmt.Sprintf("backend type %q is not supported", block.Labels[0]),
					Subject:  block.TypeRange.Ptr(),
				})
			}
		}
	}
	return diags
}

func (app *App) decodeProviderBlocks(blocks hcl.Blocks) hcl.Diagnostics {
	app.providers = make(map[string]provider.Provider, 0)
	app.providerParameters = make(provider.ProviderParameters, 0)
	var diags hcl.Diagnostics
	for _, block := range blocks {
		params := make(map[string]cty.Value)
		pp := &provider.ProviderParameter{
			Type: block.Labels[0],
			Name: "default",
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
				params[name] = value
			}
		}
		if diags.HasErrors() {
			continue
		}
		if err := pp.SetParams(params); err != nil {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  `Provider creation failed`,
				Detail:   err.Error(),
				Subject:  block.TypeRange.Ptr(),
			})
			continue
		}
		provider, err := provider.NewProvider(pp)
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
	app.queries = make(map[string]provider.Query, 0)
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
	for _, block := range blocks {
		rule := app.NewRule(block.Labels[0])
		diags = diags.Extend(rule.DecodeBody(block.Body, app.evalCtx))
		app.rules = append(app.rules, rule)
	}
	// priority sort, bigger is first
	sort.SliceStable(app.rules, func(i, j int) bool {
		return app.rules[i].Priority() > app.rules[j].Priority()
	})
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

type LoadPluginConfig struct {
	PluginName string `cty:"-"`
	Command    string `cty:"cmd"`
	SyncOutput bool   `cty:"sync_output"`
}

func (app *App) LoadPlugin(ctx context.Context, cfg *LoadPluginConfig) error {
	if cfg.Command == "" {
		return fmt.Errorf("plugin cmd is empty")
	}
	if cfg.PluginName == "" {
		return fmt.Errorf("plugin name is empty")
	}
	f, clenup, err := plugin.NewRemoteProviderFactory(cfg.PluginName, cfg.Command, cfg.SyncOutput)
	if clenup != nil {
		app.cleanupFuncs = append(app.cleanupFuncs, clenup)
	}
	if err != nil {
		return fmt.Errorf("failed load plugin: %w", err)
	}
	return provider.RegisterProviderWithError(cfg.PluginName, f.NewProvider)
}

func WebhookFromEvalContext(evalCtx *hcl.EvalContext) (*WebhookBody, error) {
	var body WebhookBody
	if evalCtx == nil {
		return nil, errors.New("evalCtx is nil")
	}
	v, ok := evalCtx.Variables[webhookHCLPrefix]
	if !ok {
		return nil, errors.New("webhook body not found")
	}
	js, _ := hclutil.DumpCTYValue(v)
	slog.Debug("dump webhook body", "detail", js)
	if err := hclutil.UnmarshalCTYValue(v, &body); err != nil {

		return nil, fmt.Errorf("failed unmarshal webhook body: %w", err)
	}
	return &body, nil
}

func (app *App) WithPrepalertFunctions(evalCtx *hcl.EvalContext) *hcl.EvalContext {
	child := provider.WithFunctions(evalCtx).NewChild()
	child.Functions = map[string]function.Function{
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
				if app.loadingConfig {
					return cty.ObjectVal(map[string]cty.Value{
						"id":   cty.NullVal(cty.String),
						"name": cty.NullVal(cty.String),
						"type": cty.NullVal(cty.String),
					}), nil
				}
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
				m, err := app.mkrSvc.GetMonitorByAlertID(context.Background(), alert.ID)
				if err != nil {
					var mkrErr *mackerel.APIError
					if errors.As(err, &mkrErr) && mkrErr.StatusCode == 404 {
						return cty.ObjectVal(map[string]cty.Value{
							"id":   cty.NullVal(cty.String),
							"name": cty.NullVal(cty.String),
							"type": cty.NullVal(cty.String),
						}), nil
					}
					return cty.UnknownVal(cty.Object(map[string]cty.Type{
						"id":   cty.String,
						"name": cty.String,
						"type": cty.String,
					})), fmt.Errorf("failed get alert: %w", err)
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

func ExpressionToString(expr hcl.Expression, evalCtx *hcl.EvalContext) (string, error) {
	value, diags := expr.Value(evalCtx)
	if diags.HasErrors() {
		return "", diags
	}
	if value.Type() != cty.String {
		return "", errors.New("expr is not string")
	}
	if value.IsNull() {
		return "", errors.New("expr is nil")
	}
	if !value.IsKnown() {
		return "", errors.New("expr is unknown")
	}
	return value.AsString(), nil
}
