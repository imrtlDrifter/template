package get

import (
	"context"
	"fmt"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"

	"github.com/google/uuid"
)

type TripService struct {
	tripRepo         tripRepository
	repositoryErrors repositoryErrors
}

func NewTripService(tripRepo tripRepository, repositoryErrors repositoryErrors) *TripService {
	return &TripService{tripRepo: tripRepo, repositoryErrors: repositoryErrors}
}

func (s *TripService) GetByID(ctx context.Context, id uuid.UUID) (*models.Trip, error) {
	trip, err := s.tripRepo.GetByID(ctx, id)
	if err != nil {
		if s.repositoryErrors.IsNotFoundError(err) {
			return nil, domainerrors.ErrTripNotFound
		}

		return nil, fmt.Errorf("get trip by id: %w", err)
	}

	return trip, nil
}
