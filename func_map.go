package prepalert

import (
	"strings"
	"text/template"
	"time"

	"github.com/lestrrat-go/strftime"
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
	"strftime": func(layout string, t time.Time) string {
		return Strftime(layout, time.Local, t)
	},
	"strftime_in_zone": StrftimeInZone,
}
var queryTemplateFuncMap template.FuncMap
var memoTemplateFuncMap template.FuncMap

func init() {
	memoTemplateFuncMap = template.FuncMap{
		"to_table": func(qr *QueryResult) string {
			return qr.ToTable()
		},
		"to_vertical": func(qr *QueryResult) string {
			return qr.ToVertical()
		},
	}
	queryTemplateFuncMap = template.FuncMap{}
	for name, f := range commonTemplateFuncMap {
		queryTemplateFuncMap[name] = f
		memoTemplateFuncMap[name] = f
	}
}
