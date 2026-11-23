// import-mongo copies newline-delimited mongoexport files into PostgreSQL.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hunterjob/hunterjob/api/internal/config"
	"github.com/hunterjob/hunterjob/api/internal/storage"
	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson"
)

type collection struct {
	name   string
	fields string
}

var collections = []collection{
	{"search_preferences", "id, user_id, role, experience, coalesce(locations, '{}'), coalesce(keywords, '{}'), coalesce(excluded_keywords, '{}'), frequency, enabled, created_at, updated_at"},
	{"job_matches", "id, user_id, job_id, dedupe_key, preference_id, match_level, coalesce(matched_keywords, '{}'), match_reason, status, matched_at, coalesce(delivered, false), delivered_at, created_at, job"},
	{"chat_connections", "id, user_id, provider, status, enabled, coalesce(external_user_id, ''), created_at, updated_at"},
	{"notification_deliveries", "id, user_id, provider, coalesce(match_ids, '{}'), status, attempts, sent_at, coalesce(error, ''), created_at"},
}

func main() {
	dir := flag.String("from", "", "directory containing one <collection>.jsonl file per collection")
	flag.Parse()
	if *dir == "" {
		log.Fatal("-from is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *dir); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, dir string) error {
	db, err := storage.OpenPostgres(ctx, config.Load().DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(784421885)"); err != nil {
			return err
		}
		for _, table := range collections {
			if err := importFile(ctx, tx, dir, table); err != nil {
				return fmt.Errorf("import %s: %w", table.name, err)
			}
		}
		return nil
	})
}

func importFile(ctx context.Context, tx pgx.Tx, dir string, table collection) error {
	file, err := os.Open(filepath.Join(dir, table.name+".jsonl"))
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	query := "INSERT INTO " + table.name + " SELECT " + table.fields +
		" FROM jsonb_populate_record(NULL::" + table.name + ", $1) ON CONFLICT (id) DO NOTHING"
	for line := 1; scanner.Scan(); line++ {
		var document bson.M
		if err := bson.UnmarshalExtJSON(scanner.Bytes(), false, &document); err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
		document["id"] = document["_id"]
		delete(document, "_id")
		payload, err := json.Marshal(document)
		if err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
		if _, err := tx.Exec(ctx, query, payload); err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
	}
	return scanner.Err()
}
