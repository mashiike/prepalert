package funcs

import (
	"strings"
	"text/template"
	"time"

	"github.com/lestrrat-go/strftime"
	"github.com/mashiike/prepalert/queryrunner"
	"github.com/olekukonko/tablewriter"
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
		"to_markdown_table": func(qr *queryrunner.QueryResult) string {
			return qr.ToTable(
				func(table *tablewriter.Table) {
					table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
					table.SetCenterSeparator("|")
					table.SetAutoFormatHeaders(false)
					table.SetAutoWrapText(false)
				},
			)
		},
		"to_borderless_table": func(qr *queryrunner.QueryResult) string {
			return qr.ToTable(
				func(table *tablewriter.Table) {
					table.SetCenterSeparator(" ")
					table.SetAutoFormatHeaders(false)
					table.SetAutoWrapText(false)
					table.SetBorder(false)
					table.SetColumnSeparator(" ")
				},
			)
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
