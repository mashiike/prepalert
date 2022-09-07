package hclconfig

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/mattn/go-isatty"
	"github.com/samber/lo"
	"golang.org/x/term"
)

func defaultConfig() *Config {
	return &Config{}
}

func Load(path string, version string) (*Config, error) {
	cfg, files, diags := load(path, version)
	if len(diags) > 0 {
		width, _, err := term.GetSize(0)
		if err != nil {
			width = 400
		}
		w := hcl.NewDiagnosticTextWriter(os.Stderr, files, uint(width), isatty.IsTerminal(os.Stdout.Fd()))
		w.WriteDiagnostics(diags)
	}
	if diags.HasErrors() {
		return nil, errors.New("config load failed")
	}
	return cfg, cfg.ValidateVersion(version)
}

func load(path string, version string) (*Config, map[string]*hcl.File, hcl.Diagnostics) {
	body, files, diags := parseFiles(path)
	if diags.HasErrors() {
		log.Printf("[debug] load `%s` failed on parse fiels", path)
		return nil, files, diags
	}
	restrictDiags := restrict(body)
	diags = append(diags, restrictDiags...)
	if diags.HasErrors() {
		log.Printf("[debug] load `%s` failed on restrict", path)
		return nil, files, diags
	}
	cfg := defaultConfig()
	ctx := newEvalContext(path, version)
	decodeDiags := gohcl.DecodeBody(body, ctx, cfg)
	diags = append(diags, decodeDiags...)
	if diags.HasErrors() {
		log.Printf("[debug] load `%s` failed on decode", path)
		return nil, files, diags
	}
	buildDiags := cfg.build(ctx)
	diags = append(diags, buildDiags...)
	if diags.HasErrors() {
		log.Printf("[debug] load `%s` failed on build", path)
		return nil, files, diags
	}
	return cfg, files, diags
}

func parseFiles(path string) (hcl.Body, map[string]*hcl.File, hcl.Diagnostics) {
	var diags hcl.Diagnostics
	if _, err := os.Stat(path); err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "path not found",
			Detail:   fmt.Sprintf("%s: %v", path, err),
		})
		return nil, nil, diags
	}
	parser := hclparse.NewParser()
	globPath := filepath.Join(path, "*.hcl")
	files, err := filepath.Glob(globPath)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "glob *.hcl failed",
			Detail:   err.Error(),
		})
		return nil, parser.Files(), diags
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
		})
		return nil, parser.Files(), diags
	}
	for _, file := range files {
		_, parseDiags := parser.ParseJSONFile(file)
		diags = append(diags, parseDiags...)
	}
	body := hcl.MergeFiles(lo.Values(parser.Files()))
	return body, parser.Files(), diags
}
