package db

import (
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations выполняет миграции базы данных PostgreSQL с помощью golang-migrate.
//
// dsn — строка подключения к базе данных PostgreSQL.
//
// Функция ищет миграции в папке ./migrations, применяет их к базе данных,
// логирует процесс и возвращает ошибку, если что-то пошло не так.
// Если миграции не требуются (ErrNoChange), сообщает об этом в логах.
func RunMigrations(dsn string) error {
	migrationsPath := "file://./migrations"
	m, err := migrate.New(migrationsPath, dsn)
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
