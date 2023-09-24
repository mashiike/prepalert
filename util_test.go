package prepalert

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtructSection(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "rule.hoge",
			input: `
hogeareaerlkwarewk

## rule.hoge
Subsection content.

#### hoge

## AnotherApp
Another section content.
`,
			expected: `## rule.hoge
Subsection content.

#### hoge
`,
		},
		{
			name: "rule.fuga",
			input: `
dareawklfarhkjakjfa
コレは手打ちの文字

## rule.fuga

ここにはruleのmemo
後ろになにもないことも
`,
			expected: `## rule.fuga

ここにはruleのmemo
後ろになにもないことも
`,
		},
		{
			name: "rule.piyo",
			input: `
## rule.piyo
ここにはruleのmemo
後ろになにもないことも
`,
			expected: `## rule.piyo
ここにはruleのmemo
後ろになにもないことも
`,
		},
		{
			name:     "rule.hoge",
			input:    ``,
			expected: ``,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := extructSection(c.input, "## "+c.name)
			require.Equal(t, c.expected, actual)
		})
	}
}
