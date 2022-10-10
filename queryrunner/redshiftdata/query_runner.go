package redshiftdata

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/redshiftdata"
	"github.com/aws/aws-sdk-go-v2/service/redshiftdata/types"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mashiike/prepalert/queryrunner"
	"github.com/samber/lo"
)

const TypeName = "redshift_data"

func init() {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName:             TypeName,
		BuildQueryRunnerFunc: BuildQueryRunner,
	})
	if err != nil {
		panic(fmt.Errorf("register redshfit_data query runner:%w", err))
	}
	log.Println("[info] load redshift_data query runner")
}

func BuildQueryRunner(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.QueryRunner, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{

			Severity: hcl.DiagError,
			Summary:  "initialize aws client",
			Detail:   fmt.Sprintf("failed load aws default config:%v", err),
			Subject:  body.MissingItemRange().Ptr(),
		})
		return nil, diags
	}
	client := redshiftdata.NewFromConfig(awsCfg)
	queryRunner := &QueryRunner{
		client: client,
		name:   name,
	}
	decodeDiags := gohcl.DecodeBody(body, ctx, queryRunner)
	diags = append(diags, decodeDiags...)
	var cluster, db, dbUser, wgName, secrets bool
	for _, attr := range queryRunner.Attrs {
		switch attr.Name {
		case "cluster_identifier":
			cluster = true
		case "database":
			db = true
		case "db_user":
			dbUser = true
		case "workgroup_name":
			wgName = true
		case "secrets_arn":
			secrets = true
		}
	}
	diag := &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Ineffective attribute combinations",
		Detail:   "A valid attribute combination in query_runner.redshift_data is one of the following patterns (secrets_arn) , (cluster_identifier, database, db_user) or (database, workgroup_name)",
		Subject:  body.MissingItemRange().Ptr(),
	}
	if secrets {
		if cluster || db || dbUser || wgName {
			log.Printf("[debug] secrets_arn is specified, but other attributes are also specified. at %s", body.MissingItemRange().String())
			diags = append(diags, diag)
			return nil, diags
		}
		return queryRunner, diags
	}
	if cluster && db && dbUser {
		if secrets || wgName {
			log.Printf("[debug] cluster_identifier, database, db_user is specified, but other attributes are also specified. at %s", body.MissingItemRange().String())
			diags = append(diags, diag)
			return nil, diags
		}
		return queryRunner, diags
	}
	if db && wgName {
		if secrets || cluster || dbUser {
			log.Printf("[debug] workgroup_name, database is specified, but other attributes are also specified. at %s", body.MissingItemRange().String())
			diags = append(diags, diag)
			return nil, diags
		}
		return queryRunner, diags
	}
	log.Printf("[debug] no valid or other combination is specified. at %s", body.MissingItemRange().String())
	diags = append(diags, diag)
	log.Printf("[debug] end redshit_data query_runner block %d error diags", len(diags.Errs()))
	return nil, diags
}

type QueryRunner struct {
	client *redshiftdata.Client
	name   string

	ClusterIdentifier *string `hcl:"cluster_identifier"`
	Database          *string `hcl:"database"`
	DbUser            *string `hcl:"db_user"`
	WorkgroupName     *string `hcl:"workgroup_name"`
	SecretsARN        *string `hcl:"secrets_arn"`

	Attrs hcl.Attributes `hcl:",body"`
}

func (r *QueryRunner) Name() string {
	return r.name
}

func (r *QueryRunner) Type() string {
	return TypeName
}

type PreparedQuery struct {
	name   string
	runner *QueryRunner

	SQL hcl.Expression `hcl:"sql"`
}

func (r *QueryRunner) Prepare(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.PreparedQuery, hcl.Diagnostics) {
	log.Printf("[debug] prepare `%s` with redshift_data query_runner", name)
	q := &PreparedQuery{
		name:   name,
		runner: r,
	}
	diags := gohcl.DecodeBody(body, ctx, q)
	if diags.HasErrors() {
		return nil, diags
	}
	/*
		if q.SQL == "" {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid SQL template",
				Detail:   "sql is empty",
				Subject:  body.MissingItemRange().Ptr(),
			})
			return nil, diags
		}
		queryTemplate, err := template.New(name).Funcs(funcs.QueryTemplateFuncMap).Parse(q.SQL)
		if err != nil {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid SQL template",
				Detail:   fmt.Sprintf("parse sql as go template: %v", err),
				Subject:  body.MissingItemRange().Ptr(),
			})
			return nil, diags
		}
		q.queryTemplate = queryTemplate
	*/
	return q, diags
}

func (q *PreparedQuery) Name() string {
	return q.name
}

func (q *PreparedQuery) Run(ctx context.Context, evalCtx *hcl.EvalContext) (*queryrunner.QueryResult, error) {
	var buf bytes.Buffer
	/*q.SQL.Value(evalCtx)
	if err := q.queryTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute query template:%w", err)
	}*/
	return q.runner.RunQuery(ctx, "prepalert-"+q.name, buf.String())
}

func (r *QueryRunner) RunQuery(ctx context.Context, stmtName string, query string) (*queryrunner.QueryResult, error) {
	reqID := "-"
	hctx, ok := queryrunner.GetQueryRunningContext(ctx)
	if ok {
		reqID = fmt.Sprintf("%d", hctx.ReqID)
	}
	log.Printf("[info][%s] start redshift data query `%s`", reqID, stmtName)
	log.Printf("[debug][%s] query: %s", reqID, query)
	executeOutput, err := r.client.ExecuteStatement(ctx, &redshiftdata.ExecuteStatementInput{
		Database:          r.Database,
		Sql:               aws.String("/* prepalert query */" + query),
		ClusterIdentifier: r.ClusterIdentifier,
		DbUser:            r.DbUser,
		SecretArn:         r.SecretsARN,
		StatementName:     aws.String(stmtName),
		WorkgroupName:     r.WorkgroupName,
	})
	if err != nil {
		return nil, fmt.Errorf("execute statement:%w", err)
	}
	queryStart := time.Now()
	waiter := &queryrunner.Waiter{
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
				return queryrunner.NewEmptyQueryResult(stmtName, query), nil
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
			return queryrunner.NewQueryResult(stmtName, query, columns, rows), nil
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
