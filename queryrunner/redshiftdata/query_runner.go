package redshiftdata

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/redshiftdata"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mashiike/prepalert/queryrunner"
)

func init() {
	err := queryrunner.Register(&queryrunner.QueryRunnerDefinition{
		TypeName:                     "redshift_data",
		RestrictQueryRunnerBlockFunc: RestrictQueryRunnerBlock,
		RestrictQueryBlockFunc:       RestrictQueryBlock,
		BuildQueryRunnerFunc:         BuildQueryRunner,
	})
	if err != nil {
		panic(fmt.Errorf("register redshfit_data query runner:%w", err))
	}
}

func RestrictQueryRunnerBlock(body hcl.Body) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "cluster_identifier",
			},
			{
				Name: "database",
			},
			{
				Name: "db_user",
			},
			{
				Name: "workgroup_name",
			},
			{
				Name: "secrets_arn",
			},
		},
	}
	content, diags := body.Content(schema)
	var cluster, db, dbUser, wgName, secrets bool
	for _, attr := range content.Attributes {
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
			return diags
		}
		return diags
	}
	if cluster && db && dbUser {
		if secrets || wgName {
			log.Printf("[debug] cluster_identifier, database, db_user is specified, but other attributes are also specified. at %s", body.MissingItemRange().String())
			diags = append(diags, diag)
			return diags
		}
		return diags
	}
	if db && wgName {
		if secrets || cluster || dbUser {
			log.Printf("[debug] workgroup_name, database is specified, but other attributes are also specified. at %s", body.MissingItemRange().String())
			diags = append(diags, diag)
			return diags
		}
		return diags
	}
	log.Printf("[debug] no valid or other combination is specified. at %s", body.MissingItemRange().String())
	diags = append(diags, diag)
	return diags
}

func RestrictQueryBlock(body hcl.Body) hcl.Diagnostics {
	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "sql",
				Required: true,
			},
		},
	}
	_, diags := body.Content(schema)
	return diags
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
	}
	decodeDiags := gohcl.DecodeBody(body, ctx, queryRunner)
	diags = append(diags, decodeDiags...)
	return queryRunner, diags
}

type QueryRunner struct {
	client *redshiftdata.Client

	ClusterIdentifier *string `hcl:"cluster_identifier"`
	Database          *string `hcl:"database"`
	DbUser            *string `hcl:"db_user"`
	WorkgroupName     *string `hcl:"workgroup_name"`
	SecretsARN        *string `hcl:"secrets_arn"`
}

type PreparedQuery struct {
	name   string
	runner *QueryRunner

	SQL string `hcl:"sql"`
}

func (r *QueryRunner) Prepare(name string, body hcl.Body, ctx *hcl.EvalContext) (queryrunner.PreparedQuery, hcl.Diagnostics) {
	q := &PreparedQuery{
		name:   name,
		runner: r,
	}
	diags := gohcl.DecodeBody(body, ctx, q)
	return q, diags
}

func (q *PreparedQuery) Name() string {
	return q.name
}

func (q *PreparedQuery) Run(ctx context.Context, data interface{}) (*queryrunner.QueryResult, error) {
	return nil, errors.New("not implemented yet")
}
