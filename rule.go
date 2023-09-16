package prepalert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/hashicorp/hcl/v2"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/mashiike/hclutil"
	"github.com/zclconf/go-cty/cty"
)

type Rule struct {
	maxGraphAnnotationDescriptionSize *int
	maxAlertMemoSize                  *int

	svcFunc             func() *MackerelService
	backend             Backend
	ruleName            string
	when                hcl.Expression
	information         hcl.Expression
	updateAlertMemo     bool
	postGraphAnnotation bool
	service             string
	dependsOnQueries    map[string]struct{}
}

const (
	webhookHCLPrefix = "webhook"
)

func NewRule(svcFunc func() *MackerelService, backend Backend, ruleName string) *Rule {
	return &Rule{
		svcFunc:          svcFunc,
		backend:          backend,
		ruleName:         ruleName,
		dependsOnQueries: make(map[string]struct{}),
	}
}

var dummyWebhook = &WebhookBody{
	Alert: &Alert{
		ID:                "dummy",
		URL:               "https://example.com",
		Status:            "ok",
		OpenedAt:          0,
		ClosedAt:          0,
		CreatedAt:         0,
		CriticalThreshold: aws.Float64(0),
		WarningThreshold:  aws.Float64(0),
		Duration:          0,
		IsOpen:            false,
		MonitorName:       "dummy",
	},
	Host: &Host{},
	Service: &Service{
		Name: "dummy",
		Roles: []*Role{
			{
				Fullname: "dummy",
			},
		},
	},
}

func (rule *Rule) DecodeBody(body hcl.Body, evalCtx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "when",
				Required: true,
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
			v, err := hclutil.MarshalCTYValue(rule.svcFunc().NewExampleWebhookBody())
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
		}
	}
	for _, block := range content.Blocks {
		switch block.Type {
		case "update_alert":
			attrs, attrDiags := block.Body.JustAttributes()
			diags = diags.Extend(attrDiags)
			if attrDiags.HasErrors() {
				continue
			}
			if memoAttr, ok := attrs["memo"]; ok {
				rule.information = memoAttr.Expr
			} else {
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "update_alert block must have memo attribute",
					Subject:  block.DefRange.Ptr(),
				})
				continue
			}
			for _, attr := range attrs {
				switch attr.Name {
				case "max_size":
					v, err := attr.Expr.Value(evalCtx)
					if err != nil {
						diags = diags.Append(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "failed evaluate max_size attribute",
							Detail:   err.Error(),
							Subject:  attr.Range.Ptr(),
						})
						continue
					}
					var maxSize int
					if err := hclutil.UnmarshalCTYValue(v, &maxSize); err != nil {
						diags = diags.Append(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "failed unmarshal max_size attribute",
							Detail:   err.Error(),
							Subject:  attr.Range.Ptr(),
						})
						continue
					}
					if maxSize <= 0 || maxSize > maxMemoSize {
						diags = diags.Append(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "max_size attribute must be positive integer",
							Detail:   fmt.Sprintf("max_size: %d", maxSize),
							Subject:  attr.Range.Ptr(),
						})
						continue
					}
					rule.maxAlertMemoSize = &maxSize
				case "memo":
				default:
					diags = diags.Append(&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  fmt.Sprintf("unknown attribute %q", attr.Name),
						Subject:  attr.Range.Ptr(),
					})
				}
			}
			rule.updateAlertMemo = true
			refarences := hclutil.VariablesReffarances(rule.information)
			for _, ref := range refarences {
				if !strings.HasPrefix(ref, "query.") {
					continue
				}
				parts := strings.Split(ref, ".")
				if len(parts) < 3 {
					continue
				}
				rule.dependsOnQueries[strings.Join(parts[:3], ".")] = struct{}{}
			}
		case "post_graph_annotation":
			attrs, attrDiags := block.Body.JustAttributes()
			diags = diags.Extend(attrDiags)
			if attrDiags.HasErrors() {
				continue
			}
			if serviceAttr, ok := attrs["service"]; ok {
				serviceVal, err := serviceAttr.Expr.Value(evalCtx)
				if err != nil {
					diags = diags.Append(&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "failed evaluate service attribute",
						Detail:   err.Error(),
						Subject:  serviceAttr.Range.Ptr(),
					})
					continue
				}
				if err := hclutil.UnmarshalCTYValue(serviceVal, &rule.service); err != nil {
					diags = diags.Append(&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  "failed unmarshal service attribute",
						Detail:   err.Error(),
						Subject:  serviceAttr.Range.Ptr(),
					})
					continue
				}
				rule.postGraphAnnotation = true
			} else {
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "post_graph_annotation block must have service attribute",
					Subject:  block.DefRange.Ptr(),
				})
				continue
			}
			for _, attr := range attrs {
				switch attr.Name {
				case "max_size":
					v, err := attr.Expr.Value(evalCtx)
					if err != nil {
						diags = diags.Append(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "failed evaluate max_size attribute",
							Detail:   err.Error(),
							Subject:  attr.Range.Ptr(),
						})
						continue
					}
					var maxSize int
					if err := hclutil.UnmarshalCTYValue(v, &maxSize); err != nil {
						diags = diags.Append(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "failed unmarshal max_size attribute",
							Detail:   err.Error(),
							Subject:  attr.Range.Ptr(),
						})
						continue
					}
					if maxSize <= 0 || maxSize > maxDescriptionSize {
						diags = diags.Append(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "max_size attribute must be positive integer",
							Detail:   fmt.Sprintf("max_size: %d", maxSize),
							Subject:  attr.Range.Ptr(),
						})
						continue
					}
					rule.maxGraphAnnotationDescriptionSize = &maxSize
				case "service":
				default:
					diags = diags.Append(&hcl.Diagnostic{
						Severity: hcl.DiagError,
						Summary:  fmt.Sprintf("unknown attribute %q", attr.Name),
						Subject:  attr.Range.Ptr(),
					})
				}
			}
		}
	}
	return nil
}

func (rule *Rule) DependsOnQueries() []string {
	queries := make([]string, 0, len(rule.dependsOnQueries))
	for query := range rule.dependsOnQueries {
		queries = append(queries, query)
	}
	return queries
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
	if match.Type() != cty.Bool {
		return false, errors.New("when expression is not bool")
	}
	return match.True(), nil
}

func (rule *Rule) Name() string {
	return rule.ruleName
}

func (rule *Rule) PostGraphAnnotation() bool {
	return rule.postGraphAnnotation
}

func (rule *Rule) UpdateAlertMemo() bool {
	return rule.updateAlertMemo
}

const (
	maxDescriptionSize = 1024
	maxMemoSize        = 80 * 1000
	defualtMaxMemoSize = 1024
)

func (rule *Rule) MaxGraphAnnotationDescriptionSize() int {
	if rule.maxGraphAnnotationDescriptionSize == nil {
		return maxDescriptionSize
	}
	if *rule.maxGraphAnnotationDescriptionSize > maxDescriptionSize {
		return maxDescriptionSize
	}
	if *rule.maxGraphAnnotationDescriptionSize <= 0 {
		return 100
	}
	return *rule.maxGraphAnnotationDescriptionSize
}

func (rule *Rule) MaxAlertMemoSize() int {
	if rule.maxAlertMemoSize == nil {
		return defualtMaxMemoSize
	}
	if *rule.maxAlertMemoSize > maxMemoSize {
		return maxMemoSize
	}
	if *rule.maxAlertMemoSize <= 0 {
		return 100
	}
	return *rule.maxAlertMemoSize
}

func (rule *Rule) Render(ctx context.Context, evalCtx *hcl.EvalContext) (string, error) {
	value, diags := rule.information.Value(evalCtx)
	if diags.HasErrors() {
		return "", diags
	}
	if value.Type() != cty.String {
		return "", errors.New("information is not string")
	}
	if value.IsNull() {
		return "", errors.New("information is nil")
	}
	if !value.IsKnown() {
		return "", errors.New("information is unknown")
	}
	return value.AsString(), nil
}

func (rule *Rule) Execute(ctx context.Context, evalCtx *hcl.EvalContext, body *WebhookBody) error {
	info, err := rule.Render(ctx, evalCtx)
	if err != nil {
		return fmt.Errorf("render information: %w", err)
	}
	slog.DebugContext(ctx, "dump infomation", "infomation", info)
	description := fmt.Sprintf("related alert: %s\n\n%s", body.Alert.URL, info)
	showDetailsURL, uploaded, err := rule.backend.Upload(
		ctx, evalCtx,
		fmt.Sprintf("%s_%s", body.Alert.ID, rule.Name()),
		strings.NewReader(description),
	)
	if err != nil {
		return fmt.Errorf("upload to backend:%w", err)
	}
	var abbreviatedMessage string = "\n..."
	var wg sync.WaitGroup
	var errNum int32
	if rule.UpdateAlertMemo() {
		memo := info
		maxSize := rule.MaxAlertMemoSize()
		if len(memo) > maxSize {
			if uploaded {
				slog.WarnContext(
					ctx,
					"alert memo is too long",
					"length", len(memo),
					"show_details_url", showDetailsURL,
				)
			} else {
				slog.WarnContext(
					ctx,
					"alert memo is too long",
					"length", len(memo),
					"full_memo", memo,
				)
			}
			if len(abbreviatedMessage) >= maxSize {
				memo = abbreviatedMessage[0:maxSize]
			} else {
				memo = memo[0:maxSize-len(abbreviatedMessage)] + abbreviatedMessage
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rule.svcFunc().UpdateAlertMemo(ctx, body.Alert.ID, memo); err != nil {
				slog.ErrorContext(ctx, "failed update alert memo", "error", err.Error())
				atomic.AddInt32(&errNum, 1)
			}
		}()
	}

	if rule.PostGraphAnnotation() {
		maxSize := rule.MaxGraphAnnotationDescriptionSize()
		if len(description) > maxSize {
			if uploaded {
				slog.WarnContext(
					ctx,
					"graph anotation description is too long",
					"length", len(description),
					"show_details_url", showDetailsURL,
				)
			} else {
				slog.WarnContext(
					ctx,
					"graph anotation description is too long",
					"length", len(description),
					"full_description", description,
				)
			}
			if len(abbreviatedMessage) >= maxSize {
				description = abbreviatedMessage[0:maxSize]
			} else {
				description = description[0:maxSize-len(abbreviatedMessage)] + abbreviatedMessage
			}
		}
		annotation := &mackerel.GraphAnnotation{
			Title:       fmt.Sprintf("prepalert alert_id=%s rule=%s", body.Alert.ID, rule.Name()),
			Description: description,
			From:        body.Alert.OpenedAt,
			To:          body.Alert.ClosedAt,
			Service:     rule.service,
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rule.svcFunc().PostGraphAnnotation(ctx, annotation); err != nil {
				slog.ErrorContext(
					ctx,
					"failed post graph annotation",
					"error", err.Error(),
				)
				atomic.AddInt32(&errNum, 1)
			}
		}()
	}
	wg.Wait()
	if errNum != 0 {
		return fmt.Errorf("has %d errors", errNum)
	}
	return nil
}
