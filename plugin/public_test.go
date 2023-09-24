package plugin

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mashiike/hclutil"
	"github.com/stretchr/testify/require"
)

func TestDecodedBody(t *testing.T) {
	queryBody := `
hoge = "fuga"
piyo = 1 + 2

tora "tiger" {
	neko = "hachi"
	uma {
		niwatori = "saru"
		animal = true
	}
}
`
	f, diags := hclsyntax.ParseConfig([]byte(queryBody), "teet.hcl", hcl.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors())
	schema := &Schema{
		Attributes: []AttributeSchema{
			{
				Name:     "hoge",
				Required: true,
			},
			{
				Name:     "piyo",
				Required: true,
			},
		},
		Blocks: []BlockSchema{
			{
				Type:         "tora",
				Required:     true,
				UniqueLabels: true,
				LabelNames:   []string{"name"},
				Body: &Schema{
					Attributes: []AttributeSchema{
						{
							Name:     "neko",
							Required: true,
						},
					},
					Blocks: []BlockSchema{
						{
							Type:     "uma",
							Required: true,
							Unique:   true,
							Body: &Schema{
								Attributes: []AttributeSchema{
									{
										Name:     "niwatori",
										Required: true,
									},
									{
										Name:     "animal",
										Required: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	var d decodedBlock
	diags = d.DecodeBody(f.Body, hclutil.NewEvalContext(), schema)
	require.False(t, diags.HasErrors())
	bs, err := d.ToJSON(hclutil.NewEvalContext())
	require.NoError(t, err)
	t.Log(string(bs))
	expected := `{
	"hoge": "fuga",
	"piyo": 3,
	"tora": {
		"tiger": {
			"neko": "hachi",
			"uma": {
				"niwatori": "saru",
				"animal": true
			}
		}
	}
}`
	require.JSONEq(t, expected, string(bs))
}
