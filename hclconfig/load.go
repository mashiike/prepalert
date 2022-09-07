package hclconfig

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/samber/lo"
)

func defaultConfig() *Config {
	return &Config{}
}

func Load(path string, version string) (*Config, error) {
	cfg, diags := load(path, version)
	for _, diag := range diags {
		switch diag.Severity {
		case hcl.DiagError:
			log.Println("[error]", diagnosticToString(diag))
		case hcl.DiagWarning:
			log.Println("[warn]", diagnosticToString(diag))
		default:
			log.Println("[info]", diagnosticToString(diag))
		}
	}
	if diags.HasErrors() {
		return nil, errors.New("config load failed")
	}
	return cfg, cfg.ValidateVersion(version)
}

func diagnosticToString(diag *hcl.Diagnostic) string {
	if diag.Subject == nil {
		return fmt.Sprintf("%s; %s", diag.Summary, diag.Detail)
	}
	return fmt.Sprintf("%s: %s; %s", diag.Subject, diag.Summary, diag.Detail)
}

func load(path string, version string) (*Config, hcl.Diagnostics) {
	body, diags := parseFiles(path)
	if diags.HasErrors() {
		log.Printf("[debug] load `%s` failed on parse fiels", path)
		return nil, diags
	}
	restrictDiags := restrict(body)
	diags = append(diags, restrictDiags...)
	if diags.HasErrors() {
		log.Printf("[debug] load `%s` failed on restrict", path)
		return nil, diags
	}
	cfg := defaultConfig()
	ctx := newEvalContext(path, version)
	decodeDiags := gohcl.DecodeBody(body, ctx, cfg)
	diags = append(diags, decodeDiags...)
	if diags.HasErrors() {
		log.Printf("[debug] load `%s` failed on decode", path)
		return nil, diags
	}
	buildDiags := cfg.build(ctx)
	diags = append(diags, buildDiags...)
	if diags.HasErrors() {
		log.Printf("[debug] load `%s` failed on build", path)
		return nil, diags
	}
	return cfg, diags
}

func parseFiles(path string) (hcl.Body, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	parser := hclparse.NewParser()
	globPath := filepath.Join(path, "*.hcl")
	files, err := filepath.Glob(globPath)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "glob *.hcl failed",
			Detail:   err.Error(),
			Subject:  &hcl.Range{Filename: globPath},
		})
		return nil, diags
	}
	for _, file := range files {
		_, parseDiags := parser.ParseHCLFile(file)
		diags = append(diags, parseDiags...)
	}
	globPath = filepath.Join(path, "*.hcl.json")
	files, err = filepath.Glob(globPath)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "glob *.hcl.json failed",
			Detail:   err.Error(),
			Subject:  &hcl.Range{Filename: globPath},
		})
		return nil, diags
	}
	for _, file := range files {
		_, parseDiags := parser.ParseJSONFile(file)
		diags = append(diags, parseDiags...)
	}
	body := hcl.MergeFiles(lo.Values(parser.Files()))
	return body, diags
}
