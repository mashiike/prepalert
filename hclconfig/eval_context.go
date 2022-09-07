package hclconfig

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func newEvalContext(basePath string, version string) *hcl.EvalContext {
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(map[string]cty.Value{
				"version": cty.StringVal(version),
			}),
		},
		Functions: map[string]function.Function{
			"file": function.New(&function.Spec{
				Params: []function.Parameter{
					{
						Name:        "path",
						Type:        cty.String,
						AllowMarked: true,
					},
				},
				Type: function.StaticReturnType(cty.String),
				Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
					pathArg, pathMarks := args[0].Unmark()
					p := pathArg.AsString()
					fp, err := os.Open(filepath.Join(basePath, p))
					if err != nil {
						err = function.NewArgError(0, err)
						return cty.UnknownVal(cty.String), err
					}
					defer fp.Close()
					bs, err := io.ReadAll(fp)
					if err != nil {
						err = function.NewArgError(0, err)
						return cty.UnknownVal(cty.String), err
					}
					return cty.StringVal(string(bs)).WithMarks(pathMarks), nil
				},
			}),
			"must_env": function.New(&function.Spec{
				Params: []function.Parameter{
					{
						Name:        "key",
						Type:        cty.String,
						AllowMarked: true,
					},
				},
				Type: function.StaticReturnType(cty.String),
				Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
					keyArg, keyMarks := args[0].Unmark()
					key := keyArg.AsString()
					value := os.Getenv(key)
					if value == "" {
						err := function.NewArgError(0, errors.New("env is not set"))
						return cty.UnknownVal(cty.String), err
					}
					return cty.StringVal(value).WithMarks(keyMarks), nil
				},
			}),
			"env": function.New(&function.Spec{
				Params: []function.Parameter{
					{
						Name:        "key",
						Type:        cty.String,
						AllowMarked: true,
					},
				},
				Type: function.StaticReturnType(cty.String),
				Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
					keyArg, keyMarks := args[0].Unmark()
					key := keyArg.AsString()
					value := os.Getenv(key)
					return cty.StringVal(value).WithMarks(keyMarks), nil
				},
			}),
		},
	}
	return ctx
}
