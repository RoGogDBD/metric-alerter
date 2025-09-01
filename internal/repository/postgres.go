package repository

import (
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dsn string) error {
	m, err := migrate.New("file://../migrations/", dsn)
	if err != nil {
		return fmt.Errorf("failed to init migrations: %v", err)
	}

	log.Println("Migration files found. Applying migrations...")

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Println("No migrations to apply. Database is up-to-date.")
		} else {
			return fmt.Errorf("failed to run migrations: %v", err)
		}
	} else {
		log.Println("Migrations applied successfully!")
	}
	return nil
}
