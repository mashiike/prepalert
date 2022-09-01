package prepalert

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshiftdata"
	"github.com/aws/aws-sdk-go-v2/service/redshiftdata/types"
	"github.com/samber/lo"
)

type RedshiftDataQueryRunner struct {
	client *redshiftdata.Client

	clusterIdentifier *string
	database          *string
	dbUser            *string
	workgroupName     *string
	secretsARN        *string
}

func newRedshiftDataQueryRunner(cfg *QueryRunnerConfig) (QueryRunner, error) {
	awsCfg, err := newAWSConfig()
	if err != nil {
		return nil, err
	}
	client := redshiftdata.NewFromConfig(awsCfg)
	runner := &RedshiftDataQueryRunner{
		client: client,
	}

	runner.clusterIdentifier = nullif(cfg.ClusterIdentifier, "")
	runner.database = nullif(cfg.Database, "")
	runner.dbUser = nullif(cfg.DBUser, "")
	runner.workgroupName = nullif(cfg.WorkgroupName, "")
	runner.secretsARN = nullif(cfg.SecretsARN, "")
	return runner, nil
}

func (r *RedshiftDataQueryRunner) Compile(cfg *QueryConfig) (CompiledQuery, error) {
	queryTemplate, err := template.New(cfg.Name).Funcs(queryTemplateFuncMap).Parse(cfg.Query)
	if err != nil {
		return nil, fmt.Errorf("parse query template:%w", err)
	}
	return &RedshiftDataCompiledQuery{
		name:          cfg.Name,
		runner:        r,
		queryTemplate: queryTemplate,
	}, nil
}

func (r *RedshiftDataQueryRunner) RunQuery(ctx context.Context, stmtName string, query string) (*QueryResult, error) {
	reqID := "-"
	hctx, ok := GetHandleContext(ctx)
	if ok {
		reqID = fmt.Sprintf("%d", hctx.ReqID)
	}
	log.Printf("[info][%s] start redshift data query `%s`", reqID, stmtName)
	log.Printf("[debug][%s] query: %s", reqID, query)
	executeOutput, err := r.client.ExecuteStatement(ctx, &redshiftdata.ExecuteStatementInput{
		Database:          r.database,
		Sql:               aws.String("/* prepalert query */" + query),
		ClusterIdentifier: r.clusterIdentifier,
		DbUser:            r.dbUser,
		SecretArn:         r.secretsARN,
		StatementName:     aws.String(stmtName),
		WorkgroupName:     r.workgroupName,
	})
	if err != nil {
		return nil, fmt.Errorf("execute statement:%w", err)
	}
	queryStart := time.Now()
	waiter := &Waiter{
		StartTime: queryStart,
		MinDelay:  100 * time.Microsecond,
		MaxDelay:  5 * time.Second,
		Timeout:   15 * time.Minute,
		Jitter:    200 * time.Millisecond,
	}
	log.Printf("[info][%s] wait redshift data query `%s` finish", reqID, stmtName)
	if err := hctx.ChangeSQSMessageVisibilityTimeout(ctx, 30*time.Second); err != nil {
		log.Println("[warn] failed change sqs message visibility timeout:", err)
	}
	for waiter.Continue(ctx) {
		elapsedTime := time.Since(queryStart)
		log.Printf("[debug][%s] wating redshift query `%s` elapsed_time=%s", reqID, stmtName, elapsedTime)
		describeOutput, err := r.client.DescribeStatement(ctx, &redshiftdata.DescribeStatementInput{
			Id: executeOutput.Id,
		})
		if err != nil {
			return nil, fmt.Errorf("describe statement:%w", err)
		}
		if elapsedTime > 10*time.Second {
			if err := hctx.ChangeSQSMessageVisibilityTimeout(ctx, 30*time.Second); err != nil {
				log.Println("[warn] failed change sqs message visibility timeout:", err)
			}
		}
		if describeOutput.Status == types.StatusStringAborted {
			return nil, fmt.Errorf("query aborted: %s", *describeOutput.Error)
		}
		if describeOutput.Status == types.StatusStringFailed {
			return nil, fmt.Errorf("query failed: %s", *describeOutput.Error)
		}
		if describeOutput.Status == types.StatusStringFinished {
			log.Printf("[info][%s] success redshift data query `%s`, elapsed_time=%s", reqID, stmtName, time.Since(queryStart))
			if !*describeOutput.HasResultSet {
				return NewEmptyQueryResult(stmtName, query), nil
			}
			p := redshiftdata.NewGetStatementResultPaginator(r.client, &redshiftdata.GetStatementResultInput{
				Id: executeOutput.Id,
			})
			var columns []string
			var rows [][]string
			for p.HasMorePages() {
				result, err := p.NextPage(ctx)
				if err != nil {
					return nil, fmt.Errorf("get statement result:%w", err)
				}
				if columns == nil {
					columns = make([]string, 0, len(result.ColumnMetadata))
					for _, c := range result.ColumnMetadata {
						columns = append(columns, *c.Label)
					}
				}
				if rows == nil {
					log.Printf("[debug][%s] total rows = %d", reqID, result.TotalNumRows)
					rows = make([][]string, 0, result.TotalNumRows)
				}
				for _, record := range result.Records {
					rows = append(rows, lo.Map(record, func(f types.Field, _ int) string {
						switch f := f.(type) {
						case *types.FieldMemberBlobValue:
							return fmt.Sprintf("%x", f.Value)
						case *types.FieldMemberBooleanValue:
							return fmt.Sprintf("%v", f.Value)
						case *types.FieldMemberDoubleValue:
							return fmt.Sprintf("%f", f.Value)
						case *types.FieldMemberIsNull:
							return ""
						case *types.FieldMemberLongValue:
							return fmt.Sprintf("%d", f.Value)
						case *types.FieldMemberStringValue:
							return f.Value
						default:
							return ""
						}
					}))
				}
			}
			return NewQueryResult(stmtName, query, columns, rows), nil
		}
	}
	log.Printf("[info][%s] timeout or cancel redshift data query `%s`", reqID, stmtName)
	cancelCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err = r.client.CancelStatement(cancelCtx, &redshiftdata.CancelStatementInput{
		Id: executeOutput.Id,
	})
	if err != nil {
		return nil, fmt.Errorf("cancel statement: %w", err)
	}
	return nil, errors.New("query timeout")
}

type RedshiftDataCompiledQuery struct {
	name          string
	runner        *RedshiftDataQueryRunner
	queryTemplate *template.Template
}

func (q *RedshiftDataCompiledQuery) Name() string {
	return q.name
}

func (q *RedshiftDataCompiledQuery) Run(ctx context.Context, data *QueryData) (*QueryResult, error) {
	var buf bytes.Buffer
	if err := q.queryTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute query template:%w", err)
	}
	return q.runner.RunQuery(ctx, "prepalert-"+q.name, buf.String())
}
