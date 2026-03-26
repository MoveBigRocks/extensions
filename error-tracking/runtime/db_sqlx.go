package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

type sqlxTxKey struct{}

type SqlxQuerier interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error)
	QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row
}

type SqlxDB struct {
	*sqlx.DB
	driver string
}

func NewSqlxDB(db *sql.DB, driver string) *SqlxDB {
	driver = strings.TrimSpace(driver)
	if driver == "" {
		driver = "postgres"
	}
	return &SqlxDB{
		DB:     sqlx.NewDb(db, driver),
		driver: driver,
	}
}

func (db *SqlxDB) Get(ctx context.Context) SqlxQuerier {
	if tx, ok := ctx.Value(sqlxTxKey{}).(*sqlx.Tx); ok {
		return &rebindQuerier{querier: tx, rebind: db.DB.Rebind}
	}
	return &rebindQuerier{querier: db.DB, rebind: db.DB.Rebind}
}

func (db *SqlxDB) Transaction(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(sqlxTxKey{}).(*sqlx.Tx); ok {
		return fn(ctx)
	}

	tx, err := db.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, sqlxTxKey{}, tx)
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

type rebindQuerier struct {
	querier SqlxQuerier
	rebind  func(string) string
}

func (q *rebindQuerier) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return q.querier.GetContext(ctx, dest, q.rebind(query), args...)
}

func (q *rebindQuerier) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return q.querier.SelectContext(ctx, dest, q.rebind(query), args...)
}

func (q *rebindQuerier) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return q.querier.ExecContext(ctx, q.rebind(query), args...)
}

func (q *rebindQuerier) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	return q.querier.QueryxContext(ctx, q.rebind(query), args...)
}

func (q *rebindQuerier) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	return q.querier.QueryRowxContext(ctx, q.rebind(query), args...)
}
