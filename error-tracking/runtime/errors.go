package sql

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/movebigrocks/platform/internal/infrastructure/stores/shared"
)

func TranslateSqlxError(err error, table string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return shared.ErrNotFound
	}

	errStr := err.Error()
	if strings.Contains(errStr, "UNIQUE constraint failed") {
		field := extractFieldFromSqliteUniqueError(errStr)
		return shared.NewUniqueViolation(table, field, nil)
	}
	if strings.Contains(errStr, "FOREIGN KEY constraint failed") {
		return shared.NewForeignKeyViolation(table, "", nil)
	}
	if strings.Contains(errStr, "NOT NULL constraint failed") {
		field := extractFieldFromSqliteNotNullError(errStr)
		return shared.NewNotNullViolation(table, field)
	}
	if strings.Contains(errStr, "CHECK constraint failed") {
		return shared.NewCheckViolation(table, "", nil)
	}
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no connection") ||
		(strings.Contains(errStr, "relation") && strings.Contains(errStr, "does not exist")) {
		return shared.ErrDatabaseUnavailable
	}
	return err
}

func extractFieldFromSqliteUniqueError(errStr string) string {
	if idx := strings.Index(errStr, "UNIQUE constraint failed:"); idx != -1 {
		remainder := strings.TrimSpace(errStr[idx+len("UNIQUE constraint failed:"):])
		if dotIdx := strings.LastIndex(remainder, "."); dotIdx != -1 {
			return strings.TrimSpace(remainder[dotIdx+1:])
		}
	}
	return ""
}

func extractFieldFromSqliteNotNullError(errStr string) string {
	if idx := strings.Index(errStr, "NOT NULL constraint failed:"); idx != -1 {
		remainder := strings.TrimSpace(errStr[idx+len("NOT NULL constraint failed:"):])
		if dotIdx := strings.LastIndex(remainder, "."); dotIdx != -1 {
			return strings.TrimSpace(remainder[dotIdx+1:])
		}
	}
	return ""
}
