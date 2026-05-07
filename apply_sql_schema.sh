 for file in sql/schema/*.sql; do
   docker compose exec -T db psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" < "$file"
 done