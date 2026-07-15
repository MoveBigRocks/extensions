// Package testdb provides isolated PostgreSQL databases for SDK consumers.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

var databaseCounter atomic.Int64

// SetupPostgres creates an isolated PostgreSQL database and returns its DSN.
// Tests are skipped when the configured admin database is unavailable.
func SetupPostgres(t testing.TB) (string, func()) {
	t.Helper()

	adminDSN := postgresAdminDSN(t)
	databaseName := fmt.Sprintf("mbr_test_%d_%d", time.Now().UnixNano(), databaseCounter.Add(1))
	testDSN, err := dsnWithDatabase(adminDSN, databaseName)
	if err != nil {
		t.Fatalf("build test postgres dsn: %v", err)
	}

	adminDB, err := sql.Open("postgres", adminDSN)
	if err != nil {
		t.Fatalf("open postgres admin database: %v", err)
	}
	adminDB.SetConnMaxLifetime(30 * time.Second)
	adminDB.SetMaxOpenConns(2)
	adminDB.SetMaxIdleConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		_ = adminDB.Close()
		t.Skipf("postgres admin database unavailable: %v", err)
	}
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", databaseName)); err != nil {
		_ = adminDB.Close()
		t.Fatalf("create postgres test database %s: %v", databaseName, err)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", databaseName)); err != nil {
			t.Logf("warning: failed to drop postgres test database %s: %v", databaseName, err)
		}
		if err := adminDB.Close(); err != nil {
			t.Logf("warning: failed to close postgres admin connection: %v", err)
		}
	}
	return testDSN, cleanup
}

func postgresAdminDSN(t testing.TB) string {
	t.Helper()
	if dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_ADMIN_DSN")); dsn != "" {
		return dsn
	}
	user := strings.TrimSpace(os.Getenv("USER"))
	if user == "" {
		t.Skip("postgres test harness requires TEST_DATABASE_ADMIN_DSN or USER")
	}
	return fmt.Sprintf("postgres://%s@127.0.0.1:5432/postgres?sslmode=disable", url.QueryEscape(user))
}

func dsnWithDatabase(adminDSN, databaseName string) (string, error) {
	parsed, err := url.Parse(adminDSN)
	if err != nil {
		return "", err
	}
	parsed.Path = "/" + databaseName
	return parsed.String(), nil
}
