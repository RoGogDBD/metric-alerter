package db

import (
	"context"
	"fmt"
	"log"

	"github.com/RoGogDBD/metric-alerter/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InitDB инициализирует пул соединений с базой данных PostgreSQL и выполняет миграции.
//
// Функция использует механизм повторных попыток (RetryWithBackoff) для подключения к базе данных
// и для выполнения миграций. В случае неудачи возвращает ошибку.
//
// ctx — контекст для управления временем жизни операций.
// dsn — строка подключения к базе данных.
//
// Возвращает указатель на пул соединений (*pgxpool.Pool) и ошибку (error), если что-то пошло не так.
func InitDB(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	err := config.RetryWithBackoff(ctx, func() error {
		var innerErr error
		pool, innerErr = pgxpool.New(ctx, dsn)
		if innerErr != nil {
			return innerErr
		}
		return pool.Ping(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db after retries: %w", err)
	}

	log.Println("Connected to PostgreSQL")

	if err := config.RetryWithBackoff(ctx, func() error {
		return RunMigrations(dsn)
	}); err != nil {
		return nil, fmt.Errorf("failed to run migrations after retries: %w", err)
	}

	return pool, nil
}
