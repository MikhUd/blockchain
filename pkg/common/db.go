package common

import (
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
	"log/slog"
	"time"
)

func ConnectToDB(ctx context.Context, dbConnectionString string) (*pgxpool.Pool, error) {
	const op = "db.Connect"
	var dbPool *pgxpool.Pool
	var err error
	retryCount := 0
	for retryCount < 5 {
		dbPool, err = pgxpool.Connect(ctx, dbConnectionString)
		if err == nil {
			break
		}
		slog.With(slog.String("op", op)).Warn("failed to connect to the database")
		time.Sleep(5 * time.Second)
		retryCount++
	}

	if err != nil {
		slog.With(slog.String("op", op)).Warn("5 connection attempts exhausted")
		return nil, err
	}

	slog.With(slog.String("op", op)).Info("connected to the database")
	return dbPool, nil
}
