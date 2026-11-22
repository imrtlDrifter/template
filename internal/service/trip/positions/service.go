package positions

import (
	"context"
	"fmt"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"

	"github.com/google/uuid"
)

type TripService struct {
	tripRepo         tripRepository
	txManager        txManager
	repositoryErrors repositoryErrors
	queue            positionQueue
	events           positionEvents
	now              func() time.Time
}

func NewTripService(tripRepo tripRepository, txManager txManager, repositoryErrors repositoryErrors, queue positionQueue, events positionEvents, now func() time.Time) *TripService {
	return &TripService{tripRepo: tripRepo, txManager: txManager, repositoryErrors: repositoryErrors, queue: queue, events: events, now: now}
}

func (s *TripService) Create(ctx context.Context, position *models.TripPosition) error {
	if err := position.Validate(s.now()); err != nil {
		return err
	}

	if err := s.txManager.Do(ctx, func(ctx context.Context) error {
		trip, err := s.tripRepo.GetByIDForUpdate(ctx, position.TripID)
		if err != nil {
			if s.repositoryErrors.IsNotFoundError(err) {
				return domainerrors.ErrTripNotFound
			}
			return fmt.Errorf("lock trip: %w", err)
		}
		if trip.Status != models.TripStatusActive {
			return domainerrors.ErrTripCompleted
		}
		if err := s.tripRepo.CreatePosition(ctx, position); err != nil {
			return fmt.Errorf("insert trip position: %w", err)
		}
		if err := s.tripRepo.UpdateLastPositionAt(ctx, position.TripID, position.RecordedAt); err != nil {
			return fmt.Errorf("update last trip position: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("save trip position: %w", err)
	}

	s.queue.Cancel(ctx, position.TripID)
	s.events.PositionSaved(ctx)
	return nil
}

func (s *TripService) List(ctx context.Context, tripID uuid.UUID) ([]models.TripPosition, error) {
	if _, err := s.tripRepo.GetByID(ctx, tripID); err != nil {
		if s.repositoryErrors.IsNotFoundError(err) {
			return nil, domainerrors.ErrTripNotFound
		}
		return nil, fmt.Errorf("get trip: %w", err)
	}

	positions, err := s.tripRepo.ListPositions(ctx, tripID)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	return positions, nil
}
