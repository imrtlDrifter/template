package get_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"
	"github.com/imrtlDrifter/template/internal/service/trip/get"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestTripServiceGetByID(t *testing.T) {
	storageErr := errors.New("database unavailable")
	for _, test := range []struct {
		repositoryErr error
		wantErr       error
		name          string
		notFound      bool
	}{
		{name: "success"},
		{name: "missing", repositoryErr: storageErr, wantErr: domainerrors.ErrTripNotFound, notFound: true},
		{name: "repository failure", repositoryErr: storageErr, wantErr: storageErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			repository := NewMocktripRepository(ctrl)
			repositoryErrors := NewMockrepositoryErrors(ctrl)
			trip := &models.Trip{ID: uuid.New(), Status: models.TripStatusActive}
			ctx := t.Context()
			repository.EXPECT().GetByID(ctx, trip.ID).Return(trip, test.repositoryErr)
			if test.repositoryErr != nil {
				repositoryErrors.EXPECT().IsNotFoundError(test.repositoryErr).Return(test.notFound)
			}
			service := get.NewTripService(repository, repositoryErrors)
			result, err := service.GetByID(ctx, trip.ID)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("GetByID() error = %v", err)
			}
			if test.wantErr != nil {
				if result != nil || !test.notFound && !strings.Contains(err.Error(), "get trip by id") {
					t.Fatalf("GetByID() = %+v, %v", result, err)
				}
				return
			}
			if result != trip {
				t.Fatalf("GetByID() returned %+v", result)
			}
		})
	}
}
