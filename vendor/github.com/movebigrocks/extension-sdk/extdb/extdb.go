package extdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type Querier interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error)
	QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)
	QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row
}

type DB struct {
	*sqlx.DB
}

type txKey struct{}

func LoadConfig() Config {
	_ = godotenv.Load()

	cfg := Config{
		DSN:             strings.TrimSpace(os.Getenv("DATABASE_DSN")),
		MaxOpenConns:    envInt("DATABASE_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    envInt("DATABASE_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: envDuration("DATABASE_CONN_MAX_LIFETIME", 5*time.Minute),
		ConnMaxIdleTime: envDuration("DATABASE_CONN_MAX_IDLE_TIME", 5*time.Minute),
	}
	if cfg.MaxOpenConns <= 0 {
		cfg.MaxOpenConns = 1
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = cfg.MaxOpenConns
	}
	return cfg
}

func OpenFromEnv() (*DB, error) {
	return Open(LoadConfig())
}

func Open(cfg Config) (*DB, error) {
	dsn := strings.TrimSpace(cfg.DSN)
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_DSN must be set")
	}
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return nil, fmt.Errorf("DATABASE_DSN must use postgres:// or postgresql://")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	maxOpenConns := cfg.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 1
	}
	maxIdleConns := cfg.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = maxOpenConns
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{DB: sqlx.NewDb(db, "postgres")}, nil
}

func (db *DB) Get(ctx context.Context) Querier {
	if db == nil || db.DB == nil {
		return rebindQuerier{}
	}
	if tx, ok := ctx.Value(txKey{}).(*sqlx.Tx); ok {
		return rebindQuerier{querier: tx, rebind: db.DB.Rebind}
	}
	return rebindQuerier{querier: db.DB, rebind: db.DB.Rebind}
}

func (db *DB) Transaction(ctx context.Context, fn func(context.Context) error) error {
	if db == nil || db.DB == nil {
		return fmt.Errorf("database is not configured")
	}
	if _, ok := ctx.Value(txKey{}).(*sqlx.Tx); ok {
		return fn(ctx)
	}

	tx, err := db.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey{}, tx)
	if err := fn(txCtx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("rollback transaction: %v (original error: %w)", rollbackErr, err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

type rebindQuerier struct {
	querier Querier
	rebind  func(string) string
}

func (q rebindQuerier) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return q.querier.GetContext(ctx, dest, q.rebind(query), args...)
}

func (q rebindQuerier) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return q.querier.SelectContext(ctx, dest, q.rebind(query), args...)
}

func (q rebindQuerier) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return q.querier.ExecContext(ctx, q.rebind(query), args...)
}

func (q rebindQuerier) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	return q.querier.NamedExecContext(ctx, q.rebind(query), arg)
}

func (q rebindQuerier) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return q.querier.QueryxContext(ctx, q.rebind(query), args...)
}

func (q rebindQuerier) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	return q.querier.QueryRowxContext(ctx, q.rebind(query), args...)
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
