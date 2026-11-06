package create

import (
	"context"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type (
	tripRepository interface {
		Create(ctx context.Context, trip *models.Trip) error
		GetByID(ctx context.Context, id uuid.UUID) (*models.Trip, error)
	}

	statusHistoryRepository interface {
		CreateStatusHistory(ctx context.Context, history *models.TripStatusHistory) error
	}

	idempotencyRepository interface {
		DeleteExpired(ctx context.Context) error
		TryCreate(ctx context.Context, key uuid.UUID, requestHash []byte, expiresAt time.Time) (bool, error)
		Get(ctx context.Context, key uuid.UUID) (*models.IdempotencyRecord, error)
		BindTrip(ctx context.Context, key, tripID uuid.UUID) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(context.Context) error) error
	}

	repositoryErrors interface {
		IsDriverBusyError(err error) bool
	}

	tripEvents interface {
		Created(ctx context.Context, trip *models.Trip)
	}
)
