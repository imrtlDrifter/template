package positions

import (
	"context"

	"github.com/imrtlDrifter/template/internal/domain/models"

	"github.com/google/uuid"
)

type tripService interface {
	Create(ctx context.Context, position *models.TripPosition) error
	List(ctx context.Context, tripID uuid.UUID) ([]models.TripPosition, error)
}
