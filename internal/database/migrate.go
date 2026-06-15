package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(db *sql.DB) error {
	// 1. Create a migration driver instance using our existing SQL connection pool
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create postgres migration driver: %w", err)
	}

	// 2. Initialize the migrator tool pointing to our local files folder
	// "file://migrations" means look inside the migrations folder in our project root
	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("migration initialization failed: %w", err)
	}

	// 3. Apply all available "Up" migrations
	fmt.Println("Checking for pending database migrations...")
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Println("Database schema is already up to date. No migrations applied.")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	fmt.Println("Database migrations applied successfully!")
	return nil
}
