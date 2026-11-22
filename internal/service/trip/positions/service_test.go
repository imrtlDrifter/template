package positions_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"
	"github.com/imrtlDrifter/template/internal/service/trip/positions"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestCreatePositionValidation(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		recordedAt  time.Time
		field       string
		coordinates models.Coordinates
	}{
		{coordinates: models.Coordinates{Latitude: 91}, recordedAt: now, field: "coordinates.latitude"},
		{coordinates: models.Coordinates{Longitude: -181}, recordedAt: now, field: "coordinates.longitude"},
		{recordedAt: now.Add(time.Minute + time.Second), field: "recorded_at"},
	} {
		t.Run(test.field, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			service := positions.NewTripService(NewMocktripRepository(ctrl), NewMocktxManager(ctrl), NewMockrepositoryErrors(ctrl), NewMockpositionQueue(ctrl), NewMockpositionEvents(ctrl), func() time.Time { return now })
			position := &models.TripPosition{TripID: uuid.New(), RecordedAt: test.recordedAt, Coordinates: test.coordinates}
			var validationErr *domainerrors.ValidationError
			if err := service.Create(t.Context(), position); !errors.As(err, &validationErr) || validationErr.Field != test.field {
				t.Fatalf("create = %v, want validation error for %s", err, test.field)
			}
		})
	}
}

func TestCreatePosition(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		wantErr error
		name    string
		status  models.TripStatus
	}{
		{name: "active", status: models.TripStatusActive},
		{name: "completed", status: models.TripStatusCompleted, wantErr: domainerrors.ErrTripCompleted},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repo := NewMocktripRepository(ctrl)
			tx := NewMocktxManager(ctrl)
			queue := NewMockpositionQueue(ctrl)
			events := NewMockpositionEvents(ctrl)
			service := positions.NewTripService(repo, tx, NewMockrepositoryErrors(ctrl), queue, events, func() time.Time { return now })
			ctx := t.Context()
			at := now.Add(time.Minute)
			point := &models.TripPosition{TripID: uuid.New(), RecordedAt: at}
			txCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			tx.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, fn func(context.Context) error) error {
				return fn(txCtx)
			})
			lock := repo.EXPECT().GetByIDForUpdate(txCtx, point.TripID).Return(&models.Trip{Status: test.status}, nil)
			if test.wantErr == nil {
				gomock.InOrder(
					lock,
					repo.EXPECT().CreatePosition(txCtx, point).Return(nil),
					repo.EXPECT().UpdateLastPositionAt(txCtx, point.TripID, at).Return(nil),
					queue.EXPECT().Cancel(ctx, point.TripID),
					events.EXPECT().PositionSaved(ctx),
				)
			}
			if err := service.Create(ctx, point); !errors.Is(err, test.wantErr) {
				t.Fatalf("create = %v, want %v", err, test.wantErr)
			}
		})
	}
}
