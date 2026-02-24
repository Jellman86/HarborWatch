package migrations

import (
	"embed"
	"fmt"
)

//go:embed sql/*.sql
var sqlFS embed.FS

type Migration struct {
	Version int
	Name    string
	SQL     string
}

func mustReadSQL(path string) string {
	b, err := sqlFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("read embedded migration %s: %v", path, err))
	}
	return string(b)
}

func defaultMigrations() []Migration {
	return []Migration{
		{
			Version: 1,
			Name:    "legacy_schema_baseline",
			SQL:     mustReadSQL("sql/0001_legacy_schema_baseline.sql"),
		},
	}
}
