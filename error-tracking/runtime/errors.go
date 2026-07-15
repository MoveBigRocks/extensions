package sql

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrDatabaseUnavailable = errors.New("database unavailable")
)

type ConstraintError struct {
	Constraint string
	Table      string
	Field      string
}

func (e *ConstraintError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s constraint violation on %s.%s", e.Constraint, e.Table, e.Field)
	}
	return fmt.Sprintf("%s constraint violation on %s", e.Constraint, e.Table)
}

func TranslateSqlxError(err error, table string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}

	errStr := err.Error()
	if strings.Contains(errStr, "UNIQUE constraint failed") {
		field := extractFieldFromSqliteUniqueError(errStr)
		return &ConstraintError{Constraint: "unique", Table: table, Field: field}
	}
	if strings.Contains(errStr, "FOREIGN KEY constraint failed") {
		return &ConstraintError{Constraint: "foreign_key", Table: table}
	}
	if strings.Contains(errStr, "NOT NULL constraint failed") {
		field := extractFieldFromSqliteNotNullError(errStr)
		return &ConstraintError{Constraint: "not_null", Table: table, Field: field}
	}
	if strings.Contains(errStr, "CHECK constraint failed") {
		return &ConstraintError{Constraint: "check", Table: table}
	}
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "no connection") ||
		(strings.Contains(errStr, "relation") && strings.Contains(errStr, "does not exist")) {
		return ErrDatabaseUnavailable
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
