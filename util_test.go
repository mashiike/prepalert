package prepalert

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtructSection(t *testing.T) {
	cases := []struct {
		name     string
		header   string
		input    string
		expected string
	}{
		{
			name:   "extruct_inner_section",
			header: "## rule.hoge",
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
			name:   "extruct_tail_section",
			header: "## rule.fuga",
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
			name:   "extruct_head_section",
			header: "## rule.piyo",
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
			name:     "empty",
			header:   "## rule.hoge",
			input:    ``,
			expected: ``,
		},
		{
			name:     "#1 header",
			header:   "# Prepalert",
			input:    "abc\n# Prepalert\n\nhoge",
			expected: "# Prepalert\n\nhoge",
		},
		{
			name:     "#1 header with #2 header",
			header:   "# Prepalert",
			input:    "abc\n# Prepalert\n\n## hoge\n\nfuga\n\n## piyo\n\nmoge",
			expected: "# Prepalert\n\n## hoge\n\nfuga\n\n## piyo\n\nmoge",
		},
		{
			name:     "#1 header with #2 header",
			header:   "# Prepalert",
			input:    "abc\n# Prepalert\n\n## hoge\n\nfuga\n\n## piyo\n\nmoge\n# Other\n\nhoge",
			expected: "# Prepalert\n\n## hoge\n\nfuga\n\n## piyo\n\nmoge",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := extructSection(c.input, c.header)
			require.Equal(t, c.expected, actual)
		})
	}
}
