package finish

import (
	"context"

	"github.com/imrtlDrifter/template/internal/domain/models"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type (
	tripEvents interface {
		Completed(ctx context.Context, trip *models.Trip)
	}

	tripRepository interface {
		GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Trip, error)
		Update(ctx context.Context, trip *models.Trip) error
	}

	statusHistoryRepository interface {
		CreateStatusHistory(ctx context.Context, history *models.TripStatusHistory) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(context.Context) error) error
	}

	repositoryErrors interface {
		IsNotFoundError(err error) bool
	}
)
