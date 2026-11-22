package finish_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"
	"github.com/imrtlDrifter/template/internal/service/trip/finish"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestTripServiceFinish(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := NewMocktripRepository(ctrl)
	historyRepository := NewMockstatusHistoryRepository(ctrl)
	txManager := NewMocktxManager(ctrl)
	repositoryErrors := NewMockrepositoryErrors(ctrl)
	events := NewMocktripEvents(ctrl)
	ctx := context.Background()
	trip := &models.Trip{ID: uuid.New(), Status: models.TripStatusActive}
	startedAt := time.Now()

	txManager.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		return fn(ctx)
	})
	getCall := repository.EXPECT().GetByIDForUpdate(ctx, trip.ID).Return(trip, nil)
	updateCall := repository.EXPECT().Update(ctx, trip).DoAndReturn(func(_ context.Context, updated *models.Trip) error {
		if updated.Status != models.TripStatusCompleted || updated.FinishedAt == nil || updated.FinishedAt.Before(startedAt) {
			t.Fatalf("domain transition was not applied before update: %+v", updated)
		}
		return nil
	})
	historyCall := historyRepository.EXPECT().CreateStatusHistory(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, history *models.TripStatusHistory) error {
		if history.TripID != trip.ID || history.FromStatus == nil || *history.FromStatus != models.TripStatusActive || history.ToStatus != models.TripStatusCompleted {
			t.Fatalf("unexpected status history: %+v", history)
		}
		return nil
	})
	gomock.InOrder(getCall, updateCall, historyCall)

	events.EXPECT().Completed(ctx, trip)
	service := finish.NewTripService(repository, historyRepository, txManager, repositoryErrors, events)
	result, err := service.Finish(ctx, trip.ID)
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if result != trip || result.FinishedAt == nil || result.Status != models.TripStatusCompleted {
		t.Fatalf("unexpected completed trip: %+v", result)
	}
}

func TestTripServiceFinishErrors(t *testing.T) {
	storageErr := errors.New("storage unavailable")
	notFoundErr := errors.New("missing row")
	tests := []struct {
		name        string
		stage       string
		expectedErr error
		context     string
	}{
		{name: "completed", stage: "completed", expectedErr: domainerrors.ErrTripCompleted, context: "finish trip transaction"},
		{name: "not found", stage: "not found", expectedErr: domainerrors.ErrTripNotFound, context: "finish trip transaction"},
		{name: "read failure", stage: "read", expectedErr: storageErr, context: "get trip for update"},
		{name: "update failure", stage: "update", expectedErr: storageErr, context: "update completed trip"},
		{name: "history failure", stage: "history", expectedErr: storageErr, context: "create completed trip status history"},
		{name: "commit failure", stage: "commit", expectedErr: storageErr, context: "finish trip transaction"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMocktripRepository(ctrl)
			historyRepository := NewMockstatusHistoryRepository(ctrl)
			txManager := NewMocktxManager(ctrl)
			repositoryErrors := NewMockrepositoryErrors(ctrl)
			events := NewMocktripEvents(ctrl)
			ctx := context.Background()
			trip := &models.Trip{ID: uuid.New(), Status: models.TripStatusActive}

			txManager.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
				if err := fn(ctx); err != nil {
					return err
				}
				if test.stage == "commit" {
					return storageErr
				}
				return nil
			})

			switch test.stage {
			case "completed":
				trip.Status = models.TripStatusCompleted
				repository.EXPECT().GetByIDForUpdate(ctx, trip.ID).Return(trip, nil)
			case "not found":
				repository.EXPECT().GetByIDForUpdate(ctx, trip.ID).Return(nil, notFoundErr)
				repositoryErrors.EXPECT().IsNotFoundError(notFoundErr).Return(true)
			case "read":
				repository.EXPECT().GetByIDForUpdate(ctx, trip.ID).Return(nil, storageErr)
				repositoryErrors.EXPECT().IsNotFoundError(storageErr).Return(false)
			case "update":
				repository.EXPECT().GetByIDForUpdate(ctx, trip.ID).Return(trip, nil)
				repository.EXPECT().Update(ctx, trip).Return(storageErr)
			case "history":
				repository.EXPECT().GetByIDForUpdate(ctx, trip.ID).Return(trip, nil)
				repository.EXPECT().Update(ctx, trip)
				historyRepository.EXPECT().CreateStatusHistory(ctx, gomock.Any()).Return(storageErr)
			case "commit":
				repository.EXPECT().GetByIDForUpdate(ctx, trip.ID).Return(trip, nil)
				repository.EXPECT().Update(ctx, trip)
				historyRepository.EXPECT().CreateStatusHistory(ctx, gomock.Any())
			}

			service := finish.NewTripService(repository, historyRepository, txManager, repositoryErrors, events)
			result, err := service.Finish(ctx, trip.ID)
			if !errors.Is(err, test.expectedErr) || !strings.Contains(err.Error(), test.context) {
				t.Fatalf("Finish() error = %v, expected %v with operation context", err, test.expectedErr)
			}
			if result != nil {
				t.Fatalf("Finish() returned a trip after failure: %+v", result)
			}
		})
	}
}
