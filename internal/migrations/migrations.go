package migrations

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// Up applies all pending migrations against the provided database.
func Up(db *sql.DB) error {
	goose.SetBaseFS(schemaFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "schema"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
