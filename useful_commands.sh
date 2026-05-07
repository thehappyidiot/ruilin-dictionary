# Commands I regulary use

# Create environment variables from a .env file
export $(grep -v '^#' .env | xargs)

# Database Migrations (the app runs these automatically on startup;
# the goose CLI is only useful for *creating* new migration files):
goose -dir ./internal/migrations/schema create new_migration_name sql

# Generate SQLC:
sqlc generate

# Go:
go test ./...
go build -o rdc cmd/api/main.go && ./rdc