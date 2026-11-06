package finish

import (
	"context"
	"fmt"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"

	"github.com/google/uuid"
)

type TripService struct {
	tripRepo          tripRepository
	statusHistoryRepo statusHistoryRepository
	txManager         txManager
	repositoryErrors  repositoryErrors
	events            tripEvents
}

func NewTripService(tripRepo tripRepository, statusHistoryRepo statusHistoryRepository, txManager txManager, repositoryErrors repositoryErrors, events tripEvents) *TripService {
	return &TripService{
		tripRepo:          tripRepo,
		statusHistoryRepo: statusHistoryRepo,
		txManager:         txManager,
		repositoryErrors:  repositoryErrors,
		events:            events,
	}
}

func (s *TripService) Finish(ctx context.Context, id uuid.UUID) (*models.Trip, error) {
	var trip *models.Trip

	err := s.txManager.Do(ctx, func(ctx context.Context) error {
		var err error
		trip, err = s.tripRepo.GetByIDForUpdate(ctx, id)
		if err != nil {
			if s.repositoryErrors.IsNotFoundError(err) {
				return domainerrors.ErrTripNotFound
			}

			return fmt.Errorf("get trip for update: %w", err)
		}

		fromStatus := trip.Status
		if err := trip.Finish(time.Now().UTC()); err != nil {
			return err
		}

		if err := s.tripRepo.Update(ctx, trip); err != nil {
			return fmt.Errorf("update completed trip: %w", err)
		}

		history := &models.TripStatusHistory{
			TripID:     trip.ID,
			FromStatus: &fromStatus,
			ToStatus:   trip.Status,
		}
		if err := s.statusHistoryRepo.CreateStatusHistory(ctx, history); err != nil {
			return fmt.Errorf("create completed trip status history: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("finish trip transaction: %w", err)
	}

	s.events.Completed(ctx, trip)
	return trip, nil
}
