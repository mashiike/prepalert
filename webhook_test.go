package prepalert_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/hclconfig"
	"github.com/mashiike/prepalert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestMarshalCTYValues(t *testing.T) {
	body := LoadJSON[prepalert.WebhookBody](t, "testdata/event.json")
	expr, diags := hclsyntax.ParseExpression([]byte("jsonencode(test_event)"), "test.hcl", hcl.InitialPos)
	require.False(t, diags.HasErrors())
	ctx := hclconfig.NewEvalContext()
	ctx = ctx.NewChild()
	ctx.Variables = map[string]cty.Value{
		"test_event": cty.ObjectVal(body.MarshalCTYValues()),
	}
	value, diags := expr.Value(ctx)
	require.False(t, diags.HasErrors())
	actual := value.AsString()
	t.Log(actual)
	expected := `{
		"alert": {
		  "closed_at": 1473130092,
		  "created_at": 1473129912693,
		  "critical_threshold": 1.9588528112516932,
		  "duration": 5,
		  "id": "2bj...",
		  "is_open": true,
		  "metric_label": "MetricName",
		  "metric_value": 2.255356387321597,
		  "monitor_name": "MonitorName",
		  "monitor_operator": "\u003e",
		  "opened_at": 1473129912,
		  "status": "critical",
		  "trigger": "monitor",
		  "url": "https://mackerel.io/orgs/.../alerts/2bj...",
		  "warning_threshold": 1.4665636369580741
		},
		"event": "alert",
		"host": {
		  "id": "22D4...",
		  "is_retired": false,
		  "memo": "",
		  "name": "app01",
		  "roles": [
			{
			  "fullname": "Service: Role",
			  "role_name": "Role",
			  "role_url": "https://mackerel.io/orgs/.../services/...",
			  "service_name": "Service",
			  "service_url": "https://mackerel.io/orgs/.../services/..."
			}
		  ],
		  "status": "working",
		  "type": "unknown",
		  "url": "https://mackerel.io/orgs/.../hosts/..."
		},
		"image_url": "alert",
		"memo": "memo....",
		"org_name": "Macker..."
	  }`
	require.JSONEq(t, expected, actual)
}
