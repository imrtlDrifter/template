package positions

import (
	"context"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type (
	tripRepository interface {
		GetByID(ctx context.Context, id uuid.UUID) (*models.Trip, error)
		GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Trip, error)
		CreatePosition(ctx context.Context, position *models.TripPosition) error
		UpdateLastPositionAt(ctx context.Context, tripID uuid.UUID, recordedAt time.Time) error
		ListPositions(ctx context.Context, tripID uuid.UUID) ([]models.TripPosition, error)
	}

	txManager interface {
		Do(ctx context.Context, fn func(context.Context) error) error
	}

	repositoryErrors interface {
		IsNotFoundError(err error) bool
	}

	positionQueue interface {
		Cancel(ctx context.Context, tripID uuid.UUID)
	}

	positionEvents interface {
		PositionSaved(ctx context.Context)
	}
)
