package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/marquisccel/banking-peak-load-prototype/internal/config"
)

const (
	numAccounts     = 100_000
	numTransactions = 1_000_000
	txBatchSize     = 10_000
	startAccountID  = 1001
)

// Status distribution: 2/3 completed, 1/6 pending, 1/12 failed
var statuses = []string{"completed", "completed", "completed", "completed", "completed", "completed", "completed", "completed", "pending", "pending", "pending", "failed"}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	dsn := cfg.DBPrimaryDSN
	if dsn == "" {
		log.Fatal("DB_PRIMARY_DSN is required")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	seedAccounts(ctx, conn)
	seedTransactions(ctx, conn)
}

func seedAccounts(ctx context.Context, conn *pgx.Conn) {
	start := time.Now()
	fmt.Printf("Seeding %d accounts...\n", numAccounts)

	rows := make([][]any, numAccounts)
	for i := range numAccounts {
		id := int64(startAccountID + i)
		rows[i] = []any{
			id,
			fmt.Sprintf("User %d", id),
			100_000 + rand.Float64()*(50_000_000-100_000),
			time.Now(),
		}
	}

	n, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"accounts"},
		[]string{"id", "name", "balance", "updated_at"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		log.Fatalf("copy accounts: %v", err)
	}
	fmt.Printf("  inserted %d accounts in %s\n", n, time.Since(start).Round(time.Millisecond))
}

func seedTransactions(ctx context.Context, conn *pgx.Conn) {
	start := time.Now()
	fmt.Printf("Seeding %d transactions in batches of %d...\n", numTransactions, txBatchSize)

	now := time.Now()
	totalInserted := int64(0)

	for batchStart := 0; batchStart < numTransactions; batchStart += txBatchSize {
		batchEnd := min(batchStart+txBatchSize, numTransactions)
		rows := make([][]any, batchEnd-batchStart)

		for i := range rows {
			idx := batchStart + i
			srcID := int64(startAccountID + rand.Intn(numAccounts))
			dstID := int64(startAccountID + rand.Intn(numAccounts-1))
			if dstID >= srcID {
				dstID++
			}
			createdAt := now.Add(-time.Duration(rand.Intn(30*24)) * time.Hour)
			rows[i] = []any{
				fmt.Sprintf("txn%022d", idx),
				srcID,
				dstID,
				1_000 + rand.Float64()*(1_000_000-1_000),
				statuses[rand.Intn(len(statuses))],
				createdAt,
			}
		}

		n, err := conn.CopyFrom(
			ctx,
			pgx.Identifier{"transactions"},
			[]string{"id", "source_account", "dest_account", "amount", "status", "created_at"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			log.Fatalf("copy transactions batch %d: %v", batchStart/txBatchSize, err)
		}
		totalInserted += n
		fmt.Printf("  %d / %d\n", totalInserted, numTransactions)
	}

	fmt.Printf("  inserted %d transactions in %s\n", totalInserted, time.Since(start).Round(time.Millisecond))
}
