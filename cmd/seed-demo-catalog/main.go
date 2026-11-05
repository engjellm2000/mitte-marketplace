package main

import (
	"context"
	"database/sql"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"mitte/internal/config"
	"mitte/internal/store"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed demo_catalog.sql
var catalogSQL string

//go:embed curated_beauty.sql
var curatedBeautySQL string

//go:embed curated_beauty_expanded.sql
var curatedBeautyExpandedSQL string

//go:embed demo_beauty_campaign.sql
var demoBeautyCampaignSQL string

const (
	demoProductsPerCategory           = 1000
	curatedBeautyProductCount         = 35
	curatedBeautyExpandedProductCount = 543
	demoBeautyCampaignCount           = 1
	demoSeedTimeout                   = 5 * time.Minute
)

func main() {
	curatedOnly := flag.Bool("curated-only", false, "seed curated beauty products without bulk synthetic products")
	flag.Parse()

	if err := config.LoadDevelopmentEnvFile(".env"); err != nil {
		log.Fatal(err)
	}
	if err := validateSeedMode(os.Getenv("APP_ENV"), os.Getenv("MITTE_ENABLE_DEMO_CATALOG_SEED")); err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if err := validateDemoDatabase(cfg.DatabaseURL); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), demoSeedTimeout)
	defer cancel()
	repository, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to development database: %v", err)
	}
	if err := repository.Migrate(ctx); err != nil {
		repository.Close()
		log.Fatalf("apply database migrations: %v", err)
	}
	if err := repository.Close(); err != nil {
		log.Fatalf("close migration connection: %v", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open development database: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to development database: %v", err)
	}
	var categoryCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories`).Scan(&categoryCount); err != nil {
		log.Fatalf("count development catalog categories: %v", err)
	}
	if categoryCount == 0 {
		log.Fatal("cannot seed demo products because the category taxonomy is empty")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatalf("begin local catalog seed transaction: %v", err)
	}
	defer tx.Rollback()
	var count int
	if !*curatedOnly {
		if err := tx.QueryRowContext(ctx, catalogSQL, demoProductsPerCategory).Scan(&count); err != nil {
			tx.Rollback()
			log.Fatalf("seed synthetic development catalog: %v", err)
		}
		expectedCount := categoryCount * demoProductsPerCategory
		if count != expectedCount {
			tx.Rollback()
			log.Fatalf("expected %d demo products (%d per category across %d categories), seeded %d; transaction rolled back", expectedCount, demoProductsPerCategory, categoryCount, count)
		}
	}
	var curatedCount int
	if err := tx.QueryRowContext(ctx, curatedBeautySQL).Scan(&curatedCount); err != nil {
		tx.Rollback()
		log.Fatalf("seed curated Open Beauty Facts products: %v", err)
	}
	if curatedCount != curatedBeautyProductCount {
		tx.Rollback()
		log.Fatalf("expected %d curated beauty products, seeded %d; transaction rolled back", curatedBeautyProductCount, curatedCount)
	}
	var expandedBeautyCount int
	if err := tx.QueryRowContext(ctx, curatedBeautyExpandedSQL).Scan(&expandedBeautyCount); err != nil {
		tx.Rollback()
		log.Fatalf("seed expanded Open Beauty Facts products: %v", err)
	}
	if expandedBeautyCount != curatedBeautyExpandedProductCount {
		tx.Rollback()
		log.Fatalf("expected %d expanded beauty products, seeded %d; transaction rolled back", curatedBeautyExpandedProductCount, expandedBeautyCount)
	}
	var campaignCount int
	if err := tx.QueryRowContext(ctx, demoBeautyCampaignSQL).Scan(&campaignCount); err != nil {
		tx.Rollback()
		log.Fatalf("seed demo beauty campaign: %v", err)
	}
	if campaignCount != demoBeautyCampaignCount {
		tx.Rollback()
		log.Fatalf("expected %d demo beauty campaign, seeded %d; transaction rolled back", demoBeautyCampaignCount, campaignCount)
	}
	if err := tx.Commit(); err != nil {
		log.Fatalf("commit local development catalog: %v", err)
	}
	if *curatedOnly {
		fmt.Printf("Applied %d curated and %d expanded beauty products plus %d clearly labeled demo campaign to the development database.\n", curatedCount, expandedBeautyCount, campaignCount)
		return
	}
	fmt.Printf("Applied %d synthetic MITTE demo products (%d per category across %d categories) and %d curated beauty products to the development database.\n", count, demoProductsPerCategory, categoryCount, curatedCount+expandedBeautyCount)
}

func validateSeedMode(environment, enabled string) error {
	if strings.TrimSpace(environment) != "development" {
		return fmt.Errorf("demo catalog seeding is only allowed when APP_ENV=development")
	}
	if strings.TrimSpace(enabled) != "1" {
		return fmt.Errorf("set MITTE_ENABLE_DEMO_CATALOG_SEED=1 to seed local development catalog products")
	}
	return nil
}

func validateDemoDatabase(databaseURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(databaseURL))
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return fmt.Errorf("demo catalog seeding requires a local PostgreSQL URL")
	}
	if !isLoopbackDatabaseHost(parsed.Hostname()) {
		return fmt.Errorf("demo catalog seeding is restricted to a local PostgreSQL database")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil || query.Has("service") {
		return fmt.Errorf("demo catalog seeding requires a direct local PostgreSQL URL")
	}
	for _, key := range []string{"host", "hostaddr"} {
		for _, host := range query[key] {
			if !isLoopbackDatabaseHost(host) {
				return fmt.Errorf("demo catalog seeding is restricted to a local PostgreSQL database")
			}
		}
	}
	return nil
}

func isLoopbackDatabaseHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
