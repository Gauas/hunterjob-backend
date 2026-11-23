# PostgreSQL cutover

The API and workers create and upgrade the schema using `DATABASE_URL`.
For an existing installation, stop the MongoDB writers while exporting and
importing so the four collections describe the same state. Back up MongoDB
and keep the backup through verification of the cutover.

```sh
mkdir -m 700 mongo-export
for collection in search_preferences job_matches chat_connections notification_deliveries; do
  mongoexport --uri "$MONGODB_URI" --collection "$collection" \
    --out "mongo-export/$collection.jsonl"
done
export DATABASE_URL='postgres://hunterjob:password@localhost:5432/hunterjob'
go run ./cmd/import-mongo -from mongo-export
```

Include the database name in the export URI. The files must be newline-delimited
Extended JSON. Use empty files for empty collections. The import is a single
transaction and retains existing IDs, delivery history and user statuses.
Re-running against the same database skips existing IDs. A conflicting unique
user or recipient key rolls back the import. Verify collection counts in
PostgreSQL before starting the API and workers with the same `DATABASE_URL`.
