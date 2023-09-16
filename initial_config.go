package prepalert

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

//go:embed initial_config.hcl
var initialConfigTemplate string

func GenerateInitialConfig(sqsQueueName string) ([]byte, error) {
	t, err := template.New("inital_config").Parse(initialConfigTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]string{
		"Version":      Version,
		"SQSQueueName": sqsQueueName,
	})
	if err != nil {
		return nil, err
	}
	file, diags := hclwrite.ParseConfig(buf.Bytes(), "config.hcl", hcl.InitialPos)
	if diags.HasErrors() {
		return nil, diags.Errs()[0]
	}
	return file.Bytes(), nil
}
