package atsruntime

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/lib/pq"

	atsmigrations "github.com/movebigrocks/platform/extensions/ats/migrations"
	platformsql "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
)

func ApplyMigrations(ctx context.Context, db *platformsql.SqlxDB) error {
	if db == nil {
		return fmt.Errorf("database is required")
	}
	if _, err := db.Get(ctx).ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pq.QuoteIdentifier(SchemaName))); err != nil {
		return fmt.Errorf("create ats schema: %w", err)
	}

	entries, err := fs.ReadDir(atsmigrations.Files, ".")
	if err != nil {
		return fmt.Errorf("read ats migrations: %w", err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		paths = append(paths, entry.Name())
	}
	sort.Strings(paths)

	for _, path := range paths {
		body, err := fs.ReadFile(atsmigrations.Files, path)
		if err != nil {
			return fmt.Errorf("read ats migration %s: %w", path, err)
		}
		sqlText := strings.ReplaceAll(string(body), "${SCHEMA_NAME}", pq.QuoteIdentifier(SchemaName))
		if _, err := db.Get(ctx).ExecContext(ctx, sqlText); err != nil {
			return fmt.Errorf("apply ats migration %s: %w", path, err)
		}
	}
	return nil
}
