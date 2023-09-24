package prepalert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Songmu/flextime"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/hclutil"
	"github.com/zclconf/go-cty/cty"
)

type Rule struct {
	app                 *App
	priority            int
	ruleName            string
	when                hcl.Expression
	updateAlert         *UpdateAlertAction
	postGraphAnnotation *PostGraphAnnotationAction
}

type UpdateAlertAction struct {
	app              *App
	ruleName         string
	memoExpr         hcl.Expression
	enable           bool
	sizeLimit        *int
	dependsOnQueries map[string]struct{}
}

type PostGraphAnnotationAction struct {
	app                       *App
	ruleName                  string
	service                   string
	additionalDescriptionExpr hcl.Expression
	enable                    bool
	dependsOnQueries          map[string]struct{}
}

const (
	webhookHCLPrefix = "webhook"
)

func (app *App) NewRule(ruleName string) *Rule {
	return &Rule{
		app:      app,
		ruleName: ruleName,
		updateAlert: &UpdateAlertAction{
			app:              app,
			ruleName:         ruleName,
			enable:           false,
			dependsOnQueries: make(map[string]struct{}),
		},
		postGraphAnnotation: &PostGraphAnnotationAction{
			app:              app,
			ruleName:         ruleName,
			enable:           false,
			dependsOnQueries: make(map[string]struct{}),
		},
	}
}
func (rule *Rule) Priority() int {
	return rule.priority
}

func registerQueryFQNs(expr hcl.Expression, dependsOn map[string]struct{}) {
	refarences := hclutil.VariablesReffarances(expr)
	for _, ref := range refarences {
		if !strings.HasPrefix(ref, "query.") {
			continue
		}
		parts := strings.Split(ref, ".")
		if len(parts) < 3 {
			continue
		}
		dependsOn[strings.Join(parts[:3], ".")] = struct{}{}
	}
}

func (rule *Rule) DecodeBody(body hcl.Body, evalCtx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "when",
				Required: true,
			},
			{
				Name: "priority",
			},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "update_alert",
			},
			{
				Type: "post_graph_annotation",
			},
		},
	}
	content, diags := body.Content(schema)
	if diags.HasErrors() {
		return diags
	}
	diags = diags.Extend(hclutil.RestrictBlock(content, []hclutil.BlockRestrictionSchema{
		{
			Type:   "update_alert",
			Unique: true,
		},
		{
			Type:   "post_graph_annotation",
			Unique: true,
		},
	}...))
	for _, attr := range content.Attributes {
		switch attr.Name {
		case "when":
			rule.when = attr.Expr
			v, err := hclutil.MarshalCTYValue(rule.app.MackerelService().NewExampleWebhookBody())
			if err != nil {
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "failed marshal dummy webhook",
					Detail:   err.Error(),
					Subject:  attr.Range.Ptr(),
				})
				continue
			}
			tempEvalCtx := hclutil.WithValue(evalCtx, webhookHCLPrefix, v)
			if _, err := rule.match(tempEvalCtx); err != nil {
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "failed evaluate when expression",
					Detail:   err.Error(),
					Subject:  attr.Range.Ptr(),
				})
				continue
			}
		case "priority":
			diags = diags.Extend(gohcl.DecodeExpression(attr.Expr, evalCtx, &rule.priority))
		}
	}
	for _, block := range content.Blocks {
		switch block.Type {
		case "update_alert":
			diags = diags.Extend(rule.updateAlert.DecodeBody(block.Body, evalCtx))
		case "post_graph_annotation":
			diags = diags.Extend(rule.postGraphAnnotation.DecodeBody(block.Body, evalCtx))
		}
	}
	return nil
}

func (action *UpdateAlertAction) DecodeBody(body hcl.Body, evalCtx *hcl.EvalContext) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return diags
	}
	for _, attr := range attrs {
		switch attr.Name {
		case "max_size":
			var maxSize int
			diags = diags.Extend(gohcl.DecodeExpression(attr.Expr, evalCtx, &maxSize))
			if diags.HasErrors() {
				continue
			}
			action.sizeLimit = &maxSize
		case "memo":
			action.memoExpr = attr.Expr
			action.enable = true
		default:
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("unknown attribute %q", attr.Name),
				Subject:  attr.Range.Ptr(),
			})
		}
	}
	if !action.enable {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "update_alert block must have memo attribute",
			Subject:  body.MissingItemRange().Ptr(),
		})
		return diags
	}
	registerQueryFQNs(action.memoExpr, action.dependsOnQueries)
	return diags
}

func (action *PostGraphAnnotationAction) DecodeBody(body hcl.Body, evalCtx *hcl.EvalContext) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return diags
	}
	for _, attr := range attrs {
		switch attr.Name {
		case "service":
			diags = diags.Extend(gohcl.DecodeExpression(attr.Expr, evalCtx, &action.service))
			if diags.HasErrors() {
				return diags
			}
			action.enable = true
		case "additional_description":
			action.additionalDescriptionExpr = attr.Expr
			registerQueryFQNs(attr.Expr, action.dependsOnQueries)
		default:
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("unknown attribute %q", attr.Name),
				Subject:  attr.Range.Ptr(),
			})
		}
	}
	if !action.enable {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "post_graph_annotation block must have service attribute",
			Subject:  body.MissingItemRange().Ptr(),
		})
		return diags
	}
	return diags
}

func (rule *Rule) DependsOnQueries() []string {
	m := make(map[string]struct{})
	for _, q := range rule.UpdateAlertAction().DependsOnQueries() {
		m[q] = struct{}{}
	}
	for _, q := range rule.PostGraphAnnotationAction().DependsOnQueries() {
		m[q] = struct{}{}
	}
	queries := make([]string, 0, len(m))
	for query := range m {
		queries = append(queries, query)
	}
	return queries
}

func (action *UpdateAlertAction) DependsOnQueries() []string {
	queries := make([]string, 0, len(action.dependsOnQueries))
	for query := range action.dependsOnQueries {
		queries = append(queries, query)
	}
	return queries
}

func (action *PostGraphAnnotationAction) DependsOnQueries() []string {
	queries := make([]string, 0, len(action.dependsOnQueries))
	for query := range action.dependsOnQueries {
		queries = append(queries, query)
	}
	return queries
}

func (rule *Rule) UpdateAlertAction() *UpdateAlertAction {
	return rule.updateAlert
}

func (rule *Rule) PostGraphAnnotationAction() *PostGraphAnnotationAction {
	return rule.postGraphAnnotation
}

func (rule *Rule) Match(evalCtx *hcl.EvalContext) bool {
	isMatch, err := rule.match(evalCtx)
	if err != nil {
		slog.Error("failed evaluate when expression", "error", err.Error())
		return false
	}
	return isMatch
}

func (rule *Rule) match(evalCtx *hcl.EvalContext) (bool, error) {
	match, err := rule.when.Value(evalCtx)
	if err != nil {
		return false, fmt.Errorf("failed evaluate when expression: %w", err)
	}
	if !match.IsKnown() {
		return false, errors.New("when expression is unknown")
	}
	if match.IsNull() {
		return false, errors.New("when expression is null")
	}
	t := match.Type()
	if t == cty.Bool {
		return match.True(), nil
	}
	if t == cty.List(cty.Bool) || t.IsTupleType() {
		// when expression is list of bool, all of them must be true
		for _, b := range match.AsValueSlice() {
			if b.Type() != cty.Bool {
				return false, errors.New("when expression allows [bool, list(bool), tuple(bool)]")
			}
			if b.False() {
				return false, nil
			}
		}
		return true, nil
	}
	return false, errors.New("when expression allows [bool, list(bool), tuple(bool)]")
}

func (rule *Rule) Name() string {
	return rule.ruleName
}

func (action *UpdateAlertAction) Enable() bool {
	return action.enable
}

func (action *PostGraphAnnotationAction) Enable() bool {
	return action.enable
}

func (rule *Rule) Execute(ctx context.Context, evalCtx *hcl.EvalContext) error {
	body, err := WebhookFromEvalContext(evalCtx)
	if err != nil {
		return err
	}
	errs := make([]error, 0, 2)
	if rule.UpdateAlertAction().Enable() {
		if err := rule.UpdateAlertAction().Execute(ctx, evalCtx, body); err != nil {
			errs = append(errs, err)
		}
	}
	if rule.PostGraphAnnotationAction().Enable() {
		if err := rule.PostGraphAnnotationAction().Execute(ctx, evalCtx, body); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (action *UpdateAlertAction) Execute(ctx context.Context, evalCtx *hcl.EvalContext, body *WebhookBody) error {
	memo, err := ExpressionToString(action.memoExpr, evalCtx)
	if err != nil {
		return fmt.Errorf("render memo: %w", err)
	}
	slog.DebugContext(ctx, "dump memo", "memo", memo)
	uploadBody := strings.NewReader(fmt.Sprintf("related alert: %s\n\n%s", body.Alert.URL, memo))
	fullTextURL, uploaded, err := action.app.Backend().Upload(
		ctx, evalCtx,
		fmt.Sprintf("%s_%s", body.Alert.ID, action.ruleName),
		uploadBody,
	)
	if err != nil {
		return fmt.Errorf("upload to backend:%w", err)
	}
	if uploaded {
		slog.DebugContext(ctx, "uploaded to backend", "full_text_url", fullTextURL)
		memo = fmt.Sprintf("Full Text URL: %s\n\n%s", fullTextURL, memo)
	}

	if action.sizeLimit != nil && len(memo) > *action.sizeLimit {
		if uploaded {
			slog.WarnContext(
				ctx,
				"alert memo is too long",
				"length", len(memo),
				"full_text_url", fullTextURL,
			)
		} else {
			slog.WarnContext(
				ctx,
				"alert memo is too long",
				"length", len(memo),
				"full_memo", memo,
			)
		}
		memo = triming(memo, *action.sizeLimit, "\n...")
	}
	err = action.app.MackerelService().UpdateAlertMemo(ctx, body.Alert.ID, "Prepalert rule."+action.ruleName, memo)
	if err != nil {
		return fmt.Errorf("update alert memo: %w", err)
	}
	return nil
}

func (action *PostGraphAnnotationAction) Execute(ctx context.Context, evalCtx *hcl.EvalContext, body *WebhookBody) error {
	descriptin := fmt.Sprintf("related alert: %s\n", body.Alert.URL)
	if action.additionalDescriptionExpr != nil {
		additionalDescription, err := ExpressionToString(action.additionalDescriptionExpr, evalCtx)
		if err != nil {
			return fmt.Errorf("render additional_description: %w", err)
		}
		descriptin += additionalDescription
	}
	slog.DebugContext(ctx, "dump description", "description", descriptin)
	to := flextime.Now().Unix()
	if body.Alert.ClosedAt != nil {
		to = *body.Alert.ClosedAt
	}
	err := action.app.MackerelService().PostGraphAnnotation(ctx, &mackerel.GraphAnnotation{
		Title:       fmt.Sprintf("prepalert alert_id=%s rule=%s", body.Alert.ID, action.ruleName),
		Description: descriptin,
		From:        body.Alert.OpenedAt,
		To:          to,
		Service:     action.service,
	})
	if err != nil {
		return fmt.Errorf("post graph annotation: %w", err)
	}
	return nil
}
