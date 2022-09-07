package wizard

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/mackerelio/mackerel-client-go"
	"github.com/manifoldco/promptui"
	"github.com/mashiike/prepalert/hclconfig"
	"github.com/samber/lo"
)

func Run(ctx context.Context, version string, apikey string, outputPath string) error {
	if _, err := os.Stat(outputPath); err != nil {
		if err := os.MkdirAll(outputPath, 0766); err != nil {
			return fmt.Errorf("can not make output dir:%w", err)
		}
	}
	service, err := selectMackerelService(apikey)
	if err != nil {
		return fmt.Errorf("can not select Mackerel service:%w", err)
	}
	sqsQueueName, err := selectSQSQueueName(ctx)
	if err != nil {
		return fmt.Errorf("can not select sqs queue name:%w", err)
	}
	bs, err := hclconfig.GenerateInitialConfig(version, sqsQueueName, service)
	if err != nil {
		return fmt.Errorf("generate config faile:%w", err)
	}
	fp, err := os.Create(filepath.Join(outputPath, "config.hcl"))
	if err != nil {
		return fmt.Errorf("create config.hcl:%w", err)
	}
	defer fp.Close()
	_, err = fp.Write(bs)
	return err
}

func selectMackerelService(apikey string) (string, error) {
	client := mackerel.NewClient(apikey)
	services, err := client.FindServices()
	if err != nil {
		return "", err
	}
	return selectItems(
		"Which Mackerel service do you use to post graph annotations?",
		lo.Map(services, func(service *mackerel.Service, _ int) string {
			return service.Name
		}),
	)
}

func selectSQSQueueName(ctx context.Context) (string, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}
	client := sqs.NewFromConfig(awsCfg)
	p := sqs.NewListQueuesPaginator(client, &sqs.ListQueuesInput{})
	items := []string{}
	for p.HasMorePages() {
		output, err := p.NextPage(ctx)
		if err != nil {
			return "", err
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
	return selectItems(
		"Please select the SQS Queue that prepalert will use",
		items,
	)
}

func selectItems(label string, items []string) (string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: items,
	}
	_, result, err := prompt.Run()
	if err != nil {
		return "", err
	}
	return result, nil
}
