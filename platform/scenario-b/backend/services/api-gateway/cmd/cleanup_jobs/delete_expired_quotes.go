// Package main provides a background job to delete expired swap quotes.
// Run hourly via cron to clean up quotes older than 1 hour (T036).
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/app"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Parse command-line flags
	retentionHours := flag.Int("retention-hours", 1, "Delete quotes older than N hours (default 1)")
	dryRun := flag.Bool("dry-run", false, "Print what would be deleted without actually deleting")
	flag.Parse()

	// Read database connection from environment
	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Calculate cutoff time (quotes older than retention period)
	cutoffTime := time.Now().UTC().Add(-time.Duration(*retentionHours) * time.Hour)
	log.Printf("Deleting swap quotes older than %s (retention: %d hours)", cutoffTime.Format(time.RFC3339), *retentionHours)

	if *dryRun {
		log.Println("DRY RUN MODE: No quotes will be deleted")
		count, err := countExpiredQuotes(db, cutoffTime)
		if err != nil {
			log.Fatalf("failed to count expired quotes: %v", err)
		}
		log.Printf("Would delete %d expired quotes", count)
		return
	}

	// Delete expired quotes
	quoteRepo := app.NewSwapQuoteRepository(db)
	deletedCount, err := quoteRepo.DeleteExpired(context.Background(), cutoffTime)
	if err != nil {
		log.Fatalf("failed to delete expired quotes: %v", err)
	}

	log.Printf("Successfully deleted %d expired swap quotes", deletedCount)
}

// countExpiredQuotes returns the number of quotes that would be deleted.
func countExpiredQuotes(db *gorm.DB, cutoffTime time.Time) (int64, error) {
	var count int64
	err := db.Model(&struct {
		TableName struct{} `gorm:"-" sql:"swap_quotes"`
	}{}).Where("created_at < ?", cutoffTime).Count(&count).Error
	return count, err
}
