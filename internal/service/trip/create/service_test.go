package create_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"
	"github.com/imrtlDrifter/template/internal/service/trip/create"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

const (
	stageFetch   = "fetch"
	stageBusy    = "busy"
	stageUnbound = "unbound"
)

type transactionContextKey struct{}

type createTest struct {
	name      string
	stage     string
	wantErr   error
	operation string
	key       bool
}

func TestTripServiceCreate(t *testing.T) {
	storageErr := errors.New("storage unavailable")
	tests := []createTest{
		{name: "success"},
		{name: "success with key", key: true},
		{name: "replay", stage: "replay", key: true},
		{name: "expired cleanup", stage: "cleanup", key: true, wantErr: storageErr, operation: "delete expired"},
		{name: "reservation", stage: "reserve", key: true, wantErr: storageErr, operation: "reserve idempotency"},
		{name: "record lookup", stage: "lookup", key: true, wantErr: storageErr, operation: "get idempotency key"},
		{name: "different body", stage: "conflict", key: true, wantErr: domainerrors.ErrIdempotencyConflict},
		{name: "unbound key", stage: stageUnbound, key: true, operation: "idempotency record has no trip"},
		{name: "trip lookup", stage: stageFetch, key: true, wantErr: storageErr, operation: "get idempotent trip"},
		{name: "insert", stage: "insert", wantErr: storageErr, operation: "create trip"},
		{name: "busy driver", stage: stageBusy, wantErr: domainerrors.ErrDriverBusy},
		{name: "history", stage: "history", wantErr: storageErr, operation: "create initial trip status history"},
		{name: "bind", stage: "bind", key: true, wantErr: storageErr, operation: "bind idempotency key"},
		{name: "commit", stage: "commit", wantErr: storageErr, operation: "create trip transaction"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMocktripRepository(ctrl)
			historyRepository := NewMockstatusHistoryRepository(ctrl)
			idempotencyRepository := NewMockidempotencyRepository(ctrl)
			txManager := NewMocktxManager(ctrl)
			repositoryErrors := NewMockrepositoryErrors(ctrl)
			events := NewMocktripEvents(ctrl)
			ctx := t.Context()
			txCtx := context.WithValue(ctx, transactionContextKey{}, true)
			req := &models.CreateTripCommand{UserID: uuid.New(), DriverID: uuid.New(), StartPoint: models.Coordinates{Latitude: 10, Longitude: 20}, EndPoint: models.Coordinates{Latitude: 30, Longitude: 40}, Price: 100}
			data, err := json.Marshal(req)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(data)
			key := uuid.New()
			var idempotencyKey *uuid.UUID
			if test.key {
				idempotencyKey = &key
			}
			replayed := test.stage == "replay" || test.stage == "lookup" || test.stage == "conflict" || test.stage == stageUnbound || test.stage == stageFetch
			var stored *models.Trip
			txManager.EXPECT().Do(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, fn func(context.Context) error) error {
				if callbackErr := fn(txCtx); callbackErr != nil {
					return callbackErr
				}
				if test.stage == "commit" {
					return storageErr
				}
				return nil
			})
			if test.wantErr != nil || test.stage == stageUnbound {
				repositoryErrors.EXPECT().IsDriverBusyError(gomock.Any()).Return(test.stage == stageBusy)
			}
			expectIdempotency(t, test, txCtx, storageErr, idempotencyRepository, hash, key, replayed)
			if replayed {
				expectReplay(test, txCtx, storageErr, repository, idempotencyRepository, req, hash, key, &stored)
			} else if test.stage != "cleanup" && test.stage != "reserve" {
				expectInsert(t, test, txCtx, storageErr, repository, historyRepository, idempotencyRepository, req, key, &stored)
			}
			if test.stage == "" {
				events.EXPECT().Created(ctx, gomock.Any()).Do(func(_ context.Context, trip *models.Trip) {
					if trip != stored {
						t.Fatal("event contains another trip")
					}
				})
			}
			service := create.NewTripService(repository, historyRepository, idempotencyRepository, txManager, repositoryErrors, events)
			trip, created, err := service.Create(ctx, req, idempotencyKey)
			if test.wantErr != nil || test.stage == stageUnbound {
				if err == nil || test.wantErr != nil && !errors.Is(err, test.wantErr) || test.operation != "" && !strings.Contains(err.Error(), test.operation) || trip != nil || created {
					t.Fatalf("Create() = %+v, %v, %v", trip, created, err)
				}
				return
			}
			if err != nil || trip != stored || created != !replayed {
				t.Fatalf("Create() = %+v, %v, %v", trip, created, err)
			}
		})
	}
}

func TestTripServiceCreateBeforeTransaction(t *testing.T) {
	for _, test := range []struct {
		name     string
		field    string
		latitude float64
		userID   uuid.UUID
	}{
		{name: "validation", userID: uuid.Nil, field: "user_id"},
		{name: "hash failure", userID: uuid.New(), latitude: math.NaN()},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			service := create.NewTripService(NewMocktripRepository(ctrl), NewMockstatusHistoryRepository(ctrl), NewMockidempotencyRepository(ctrl), NewMocktxManager(ctrl), NewMockrepositoryErrors(ctrl), NewMocktripEvents(ctrl))
			trip, created, err := service.Create(t.Context(), &models.CreateTripCommand{UserID: test.userID, DriverID: uuid.New(), StartPoint: models.Coordinates{Latitude: test.latitude}}, nil)
			if err == nil || trip != nil || created {
				t.Fatalf("Create() = %+v, %v, %v", trip, created, err)
			}
			if test.field != "" {
				var validationErr *domainerrors.ValidationError
				if !errors.As(err, &validationErr) || validationErr.Field != test.field {
					t.Fatalf("unexpected validation error: %v", err)
				}
			} else if !strings.Contains(err.Error(), "marshal create trip request") {
				t.Fatalf("missing operation context: %v", err)
			}
		})
	}
}

func expectIdempotency(t *testing.T, test createTest, txCtx context.Context, storageErr error, idempotencyRepository *MockidempotencyRepository, hash [32]byte, key uuid.UUID, replayed bool) {
	if test.key {
		var cleanupErr error
		if test.stage == "cleanup" {
			cleanupErr = storageErr
		}
		idempotencyRepository.EXPECT().DeleteExpired(txCtx).Return(cleanupErr)
		if cleanupErr == nil {
			idempotencyRepository.EXPECT().TryCreate(txCtx, key, hash[:], gomock.Any()).DoAndReturn(func(_ context.Context, _ uuid.UUID, _ []byte, expiresAt time.Time) (bool, error) {
				if until := time.Until(expiresAt); until < 24*time.Hour-time.Second || until > 24*time.Hour {
					t.Fatalf("unexpected TTL: %s", until)
				}
				if test.stage == "reserve" {
					return false, storageErr
				}
				return !replayed, nil
			})
		}
	}
}

func expectReplay(test createTest, txCtx context.Context, storageErr error, repository *MocktripRepository, idempotencyRepository *MockidempotencyRepository, req *models.CreateTripCommand, hash [32]byte, key uuid.UUID, stored **models.Trip) {
	tripID := uuid.New()
	record := &models.IdempotencyRecord{Key: key, RequestHash: hash[:], TripID: &tripID}
	if test.stage == "conflict" {
		record.RequestHash = []byte("another request")
	}
	if test.stage == stageUnbound {
		record.TripID = nil
	}
	var lookupErr error
	if test.stage == "lookup" {
		lookupErr = storageErr
	}
	idempotencyRepository.EXPECT().Get(txCtx, key).Return(record, lookupErr)
	if test.stage == "replay" || test.stage == stageFetch {
		*stored = &models.Trip{ID: tripID, UserID: req.UserID, DriverID: req.DriverID, Status: models.TripStatusActive}
		var fetchErr error
		if test.stage == stageFetch {
			fetchErr = storageErr
		}
		repository.EXPECT().GetByID(txCtx, tripID).Return(*stored, fetchErr)
	}
}

func expectInsert(t *testing.T, test createTest, txCtx context.Context, storageErr error, repository *MocktripRepository, historyRepository *MockstatusHistoryRepository, idempotencyRepository *MockidempotencyRepository, req *models.CreateTripCommand, key uuid.UUID, stored **models.Trip) {
	repository.EXPECT().Create(txCtx, gomock.Any()).DoAndReturn(func(_ context.Context, trip *models.Trip) error {
		if trip.ID == uuid.Nil || trip.UserID != req.UserID || trip.DriverID != req.DriverID || trip.StartPoint != req.StartPoint || trip.EndPoint != req.EndPoint || trip.Price != req.Price || trip.Status != models.TripStatusActive || trip.StartedAt.IsZero() || trip.FinishedAt != nil {
			t.Fatalf("unexpected new trip: %+v", trip)
		}
		*stored = trip
		if test.stage == "insert" || test.stage == stageBusy {
			return storageErr
		}
		return nil
	})
	if test.stage != "insert" && test.stage != stageBusy {
		historyRepository.EXPECT().CreateStatusHistory(txCtx, gomock.Any()).DoAndReturn(func(_ context.Context, history *models.TripStatusHistory) error {
			if history.TripID != (*stored).ID || history.FromStatus != nil || history.ToStatus != models.TripStatusActive {
				t.Fatalf("unexpected history: %+v", history)
			}
			if test.stage == "history" {
				return storageErr
			}
			return nil
		})
		if test.key {
			idempotencyRepository.EXPECT().BindTrip(txCtx, key, gomock.Any()).DoAndReturn(func(_ context.Context, _, tripID uuid.UUID) error {
				if tripID != (*stored).ID {
					t.Fatal("key bound to another trip")
				}
				if test.stage == "bind" {
					return storageErr
				}
				return nil
			})
		}
	}
}
