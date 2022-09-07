package wizard

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Songmu/flextime"
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
	client := mackerel.NewClient(apikey)
	service, err := selectMackerelService(client)
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
	if _, err = fp.Write(bs); err != nil {
		return err
	}
	if err := generateSampleEvent(client, outputPath); err != nil {
		return err
	}
	f := ""
	if apikey == "" {
		f = "--mackerel-apikey <your Mackerel api key> "
	}
	fmt.Println("\nTry running the following two commands in a separate terminal:")
	fmt.Printf("\n$ prepalert --config %s %srun --mode webhook\n", outputPath, f)
	fmt.Printf("$ prepalert --config %s %srun --mode worker\n", outputPath, f)
	fmt.Println("\nWhen performing a local operating environment, request the following")
	fmt.Printf("\n"+`$ cat %s | curl -d @- -H "Content-Type: application/json" http://localhost:8080`, filepath.Join(outputPath, "event.json"))
	fmt.Println("\nHave fun then.")
	return err
}

func selectMackerelService(client *mackerel.Client) (string, error) {
	services, err := client.FindServices()
	if err != nil {
		services = make([]*mackerel.Service, 0)
	}
	return selectItems(
		"Which Mackerel service do you use to post graph annotations?",
		lo.Map(services, func(service *mackerel.Service, _ int) string {
			return service.Name
		}),
	)
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

//go:embed sample_event.json.tpl
var sampleEventTemplate string

func generateSampleEvent(client *mackerel.Client, outputPath string) error {
	t, err := template.New("sample_event").Parse(sampleEventTemplate)
	if err != nil {
		return err
	}
	now := flextime.Now()
	alert := &mackerel.Alert{
		ID:       "dummyAlertID",
		Status:   "OK",
		OpenedAt: now.Add(-15 * time.Minute).Unix(),
		ClosedAt: now.Add(-1 * time.Minute).Unix(),
		Value:    2.255356387321597,
	}
	monitorName := "dummyMonitor"
	orgName := "dummyOrg"
	if org, err := client.GetOrg(); err == nil {
		orgName = org.Name
	}
	resp, err := client.FindWithClosedAlerts()
	if err == nil {
		for _, a := range resp.Alerts {
			if a.Status != "OK" {
				continue
			}
			alert = a
			if monitor, err := client.GetMonitor(a.MonitorID); err == nil {
				monitorName = monitor.MonitorName()
				monitor.MonitorType()
			}
			break
		}
	}
	createdAt := alert.OpenedAt * 1000
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]interface{}{
		"OpenedAt":    alert.OpenedAt,
		"ClosedAt":    alert.ClosedAt,
		"CreatedAt":   createdAt,
		"AlertID":     alert.ID,
		"MetricValue": alert.Value,
		"OrgName":     orgName,
		"MonitorName": monitorName,
	})
	if err != nil {
		return err
	}
	fp, err := os.Create(filepath.Join(outputPath, "event.json"))
	if err != nil {
		return fmt.Errorf("create event.json%w", err)
	}
	defer fp.Close()
	_, err = fp.Write(buf.Bytes())
	return err
}
