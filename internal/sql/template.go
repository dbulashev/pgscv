package sql

import (
	"bytes"
	"embed"
	"github.com/cherts/pgscv/internal/collector"
	"log"
	"text/template"
)

//go:embed collect/*.sql
var content embed.FS

type consts struct {
	PostgresV95 int
	PostgresV96 int
	PostgresV10 int
	PostgresV11 int
	PostgresV12 int
	PostgresV13 int
	PostgresV14 int
	PostgresV15 int
	PostgresV16 int
	PostgresV17 int
}

var constants = consts{
	PostgresV95: collector.PostgresV95,
	PostgresV96: collector.PostgresV96,
	PostgresV10: collector.PostgresV10,
	PostgresV11: collector.PostgresV11,
	PostgresV12: collector.PostgresV12,
	PostgresV13: collector.PostgresV13,
	PostgresV14: collector.PostgresV14,
	PostgresV15: collector.PostgresV15,
	PostgresV16: collector.PostgresV16,
	PostgresV17: collector.PostgresV17,
}

func GetQuery(config collector.Config, templ string) (string, error) {
	data, err := content.ReadFile(templ)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(templ).Parse(string(data))
	if err != nil {
		return "", err
	}

	params := map[string]interface{}{
		"Config": config,
		"Const":  constants,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, params)
	if err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}

	return buf.String(), nil
}
