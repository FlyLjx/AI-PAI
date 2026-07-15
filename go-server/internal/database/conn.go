package database

import (
	"context"
	"database/sql"
)

type DB struct {
	raw *sql.DB
}

type Tx struct {
	raw *sql.Tx
}

type Row struct {
	raw *sql.Row
}

func Wrap(raw *sql.DB) *DB {
	if raw == nil {
		return nil
	}
	return &DB{raw: raw}
}

func (db *DB) Raw() *sql.DB {
	if db == nil {
		return nil
	}
	return db.raw
}

func (db *DB) Close() error {
	if db == nil || db.raw == nil {
		return nil
	}
	return db.raw.Close()
}

func (db *DB) PingContext(ctx context.Context) error {
	return db.raw.PingContext(ctx)
}

func (db *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.raw.ExecContext(ctx, NormalizeQuery(query), args...)
}

func (db *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.raw.QueryContext(ctx, NormalizeQuery(query), args...)
}

func (db *DB) QueryRowContext(ctx context.Context, query string, args ...any) *Row {
	return &Row{raw: db.raw.QueryRowContext(ctx, NormalizeQuery(query), args...)}
}

func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.raw.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{raw: tx}, nil
}

func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.raw.ExecContext(ctx, NormalizeQuery(query), args...)
}

func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return tx.raw.QueryContext(ctx, NormalizeQuery(query), args...)
}

func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *Row {
	return &Row{raw: tx.raw.QueryRowContext(ctx, NormalizeQuery(query), args...)}
}

func (tx *Tx) Commit() error {
	return tx.raw.Commit()
}

func (tx *Tx) Rollback() error {
	return tx.raw.Rollback()
}

func (row *Row) Scan(dest ...any) error {
	return row.raw.Scan(dest...)
}
