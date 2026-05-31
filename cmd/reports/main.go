// Reports CLI: read-only view of incoming moderation reports.
// Run: go run ./cmd/reports        — list pending
//      go run ./cmd/reports all    — include reviewed/dismissed
//      go run ./cmd/reports dismiss <report-id>   — close without action
//      go run ./cmd/reports actioned <report-id>  — mark as taken-action
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/seeu/backend/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	args := os.Args[1:]
	switch {
	case len(args) == 0:
		listReports(ctx, pool, false)
	case len(args) == 1 && args[0] == "all":
		listReports(ctx, pool, true)
	case len(args) == 2 && args[0] == "dismiss":
		updateStatus(ctx, pool, args[1], "dismissed")
	case len(args) == 2 && args[0] == "actioned":
		updateStatus(ctx, pool, args[1], "actioned")
	default:
		fmt.Fprintln(os.Stderr, "usage: reports [all|dismiss <id>|actioned <id>]")
		os.Exit(2)
	}
}

func listReports(ctx context.Context, pool *pgxpool.Pool, includeAll bool) {
	q := `
		SELECT r.id, r.target_type, r.target_id, r.reason, r.details, r.status,
		       r.created_at, u.username
		FROM reports r
		JOIN users u ON u.id = r.reporter_id`
	if !includeAll {
		q += ` WHERE r.status = 'pending'`
	}
	q += ` ORDER BY r.created_at DESC LIMIT 200`

	rows, err := pool.Query(ctx, q)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	defer rows.Close()

	fmt.Printf("%-38s %-10s %-12s %-12s %-10s %-19s %s\n",
		"ID", "TYPE", "REASON", "STATUS", "REPORTER", "WHEN", "DETAILS")
	fmt.Println(strings.Repeat("-", 130))

	count := 0
	for rows.Next() {
		var id, ttype, tid, reason, details, status, reporter string
		var created time.Time
		if err := rows.Scan(&id, &ttype, &tid, &reason, &details, &status, &created, &reporter); err != nil {
			log.Fatalf("scan: %v", err)
		}
		fmt.Printf("%-38s %-10s %-12s %-12s %-10s %-19s %s\n",
			id, ttype, reason, status, truncate(reporter, 10),
			created.Local().Format("2006-01-02 15:04:05"), truncate(details, 60))
		count++
	}
	fmt.Printf("\n%d report(s).\n", count)
}

func updateStatus(ctx context.Context, pool *pgxpool.Pool, id, status string) {
	tag, err := pool.Exec(ctx,
		`UPDATE reports SET status = $1, reviewed_at = NOW() WHERE id = $2`,
		status, id)
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	if tag.RowsAffected() == 0 {
		fmt.Println("no such report")
		os.Exit(1)
	}
	fmt.Printf("report %s -> %s\n", id, status)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
