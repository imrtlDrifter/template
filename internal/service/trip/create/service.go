package create

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"

	"github.com/google/uuid"
)

type TripService struct {
	tripRepo          tripRepository
	statusHistoryRepo statusHistoryRepository
	idempotencyRepo   idempotencyRepository
	txManager         txManager
	repositoryErrors  repositoryErrors
	events            tripEvents
}

func NewTripService(tripRepo tripRepository, statusHistoryRepo statusHistoryRepository, idempotencyRepo idempotencyRepository, txManager txManager, repositoryErrors repositoryErrors, events tripEvents) *TripService {
	return &TripService{
		tripRepo:          tripRepo,
		statusHistoryRepo: statusHistoryRepo,
		idempotencyRepo:   idempotencyRepo,
		txManager:         txManager,
		repositoryErrors:  repositoryErrors,
		events:            events,
	}
}

func (s *TripService) Create(ctx context.Context, req *models.CreateTripCommand, idempotencyKey *uuid.UUID) (*models.Trip, bool, error) {
	if err := req.Validate(); err != nil {
		return nil, false, err
	}

	trip := &models.Trip{
		ID:         uuid.New(),
		UserID:     req.UserID,
		DriverID:   req.DriverID,
		StartPoint: req.StartPoint,
		EndPoint:   req.EndPoint,
		Price:      req.Price,
		Status:     models.TripStatusActive,
		StartedAt:  time.Now().UTC(),
	}

	requestHash, hashErr := createRequestHash(req)
	if hashErr != nil {
		return nil, false, hashErr
	}

	created := false
	transactionErr := s.txManager.Do(ctx, func(ctx context.Context) error {
		if idempotencyKey != nil {
			if err := s.idempotencyRepo.DeleteExpired(ctx); err != nil {
				return fmt.Errorf("delete expired idempotency keys: %w", err)
			}

			reserved, err := s.idempotencyRepo.TryCreate(ctx, *idempotencyKey, requestHash, time.Now().UTC().Add(24*time.Hour))
			if err != nil {
				return fmt.Errorf("reserve idempotency key: %w", err)
			}

			if !reserved {
				record, err := s.idempotencyRepo.Get(ctx, *idempotencyKey)
				if err != nil {
					return fmt.Errorf("get idempotency key: %w", err)
				}

				if !bytes.Equal(record.RequestHash, requestHash) {
					return domainerrors.ErrIdempotencyConflict
				}

				if record.TripID == nil {
					return errors.New("idempotency record has no trip")
				}

				trip, err = s.tripRepo.GetByID(ctx, *record.TripID)
				if err != nil {
					return fmt.Errorf("get idempotent trip: %w", err)
				}

				return nil
			}
		}

		if err := s.tripRepo.Create(ctx, trip); err != nil {
			return fmt.Errorf("create trip: %w", err)
		}

		history := &models.TripStatusHistory{
			TripID:   trip.ID,
			ToStatus: models.TripStatusActive,
		}
		if err := s.statusHistoryRepo.CreateStatusHistory(ctx, history); err != nil {
			return fmt.Errorf("create initial trip status history: %w", err)
		}

		if idempotencyKey != nil {
			if err := s.idempotencyRepo.BindTrip(ctx, *idempotencyKey, trip.ID); err != nil {
				return fmt.Errorf("bind idempotency key to trip: %w", err)
			}
		}

		created = true
		return nil
	})
	if transactionErr != nil {
		if s.repositoryErrors.IsDriverBusyError(transactionErr) {
			return nil, false, domainerrors.ErrDriverBusy
		}

		return nil, false, fmt.Errorf("create trip transaction: %w", transactionErr)
	}

	if created {
		s.events.Created(ctx, trip)
	}
	return trip, created, nil
}

func createRequestHash(req *models.CreateTripCommand) ([]byte, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal create trip request: %w", err)
	}

	sum := sha256.Sum256(data)
	return sum[:], nil
}
