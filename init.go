package prepalert

import (
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/manifoldco/promptui"
	"github.com/samber/lo"
)

func (app *App) Init(ctx context.Context, outputPath string) error {
	if _, err := os.Stat(outputPath); err != nil {
		if err := os.MkdirAll(outputPath, 0766); err != nil {
			return fmt.Errorf("can not make output dir:%w", err)
		}
	}
	sqsQueueName, err := selectSQSQueueName(ctx)
	if err != nil {
		return fmt.Errorf("can not select sqs queue name:%w", err)
	}
	orgName := "<your org name>"
	if o, err := app.mkrSvc.client.GetOrg(); err == nil {
		orgName = o.Name
	}
	bs, err := GenerateInitialConfig(sqsQueueName, orgName)
	if err != nil {
		return fmt.Errorf("generate config faile:%w", err)
	}
	fp, err := os.Create(filepath.Join(outputPath, "config.hcl"))
	if err != nil {
		return fmt.Errorf("create config.hcl:%w", err)
	}
	defer fp.Close()
	if _, err = fp.Write(bs); err != nil {
		return err
	}
	f := ""
	if os.Getenv("MACKEREL_APIKEY") == "" {
		f = "--mackerel-apikey <your Mackerel api key> "
	}
	fmt.Println("\nTry running the following commands:")
	fmt.Printf("\n$ prepalert --config %s %sexec <alert-id>\n", outputPath, f)
	return err
}

func selectSQSQueueName(ctx context.Context) (string, error) {
	label := "Which SQS Queue do you use?"
	items := []string{}
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return selectItems(label, items)
	}
	client := sqs.NewFromConfig(awsCfg)
	p := sqs.NewListQueuesPaginator(client, &sqs.ListQueuesInput{})

	for p.HasMorePages() {
		output, err := p.NextPage(ctx)
		if err != nil {
			break
		}
		items = append(items, lo.FilterMap(output.QueueUrls, func(urlStr string, _ int) (string, bool) {
			u, err := url.Parse(urlStr)
			if err != nil {
				return "", false
			}
			parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
			if len(parts) != 2 {
				return "", false
			}
			return parts[1], true
		})...)
	}
	return selectItems(label, items)
}

func selectItems(label string, items []string) (string, error) {
	if len(items) == 0 {
		prompt := promptui.Prompt{
			Label: label,
		}
		return prompt.Run()
	}
	prompt := promptui.SelectWithAdd{
		Label: label,
		Items: items,
	}
	_, result, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return result, nil
}
