package funcs

import (
	"strings"
	"text/template"
	"time"

	"github.com/lestrrat-go/strftime"
	"github.com/mashiike/prepalert/queryrunner"
)

func StrftimeInZone(layout string, zone string, t time.Time) string {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		loc, _ = time.LoadLocation("UTC")
	}
	return Strftime(layout, loc, t)
}

func Strftime(layout string, loc *time.Location, t time.Time) string {
	t = t.In(loc)
	if strings.ContainsRune(layout, '%') {
		f, err := strftime.New(layout)
		if err != nil {
			panic(err)
		}
		return f.FormatString(t)
	}
	if strings.EqualFold("rfc3399", layout) {
		return t.Format(time.RFC3339)
	}
	return t.Format(layout)
}

var commonTemplateFuncMap = template.FuncMap{
	"to_time": func(seconds int64) time.Time {
		return time.Unix(seconds, 0)

	},
	"add_time": func(d string, t time.Time) (time.Time, error) {
		duration, err := time.ParseDuration(d)
		if err != nil {
			return time.Time{}, err
		}
		return t.Add(duration), nil
	},
	"strftime": func(layout string, t time.Time) string {
		return Strftime(layout, time.Local, t)
	},
	"strftime_in_zone": StrftimeInZone,
}
var QueryTemplateFuncMap template.FuncMap
var InfomationTemplateFuncMap template.FuncMap

func init() {
	InfomationTemplateFuncMap = template.FuncMap{
		"to_table": func(qr *queryrunner.QueryResult) string {
			return qr.ToTable()
		},
		"to_vertical": func(qr *queryrunner.QueryResult) string {
			return qr.ToVertical()
		},
		"to_json": func(qr *queryrunner.QueryResult) string {
			return qr.ToJSON()
		},
	}
	QueryTemplateFuncMap = template.FuncMap{}
	for name, f := range commonTemplateFuncMap {
		QueryTemplateFuncMap[name] = f
		InfomationTemplateFuncMap[name] = f
	}
}
