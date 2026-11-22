package finish_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"
	api "github.com/imrtlDrifter/template/internal/generated"
	"github.com/imrtlDrifter/template/internal/handlers/v1/trip/finish"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestFinishTrip(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   string
		status int
	}{
		{name: "success", status: http.StatusOK},
		{name: "not found", err: domainerrors.ErrTripNotFound, code: "trip_not_found", status: http.StatusNotFound},
		{name: "completed", err: domainerrors.ErrTripCompleted, code: "trip_completed", status: http.StatusConflict},
		{name: "internal", err: errors.New("secret connection string"), code: "internal_error", status: http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewMocktripService(gomock.NewController(t))
			trip := &models.Trip{ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(), Price: 100, Status: models.TripStatusCompleted, StartedAt: time.Now().UTC()}
			finished := trip.StartedAt.Add(time.Minute)
			trip.FinishedAt = &finished
			ctx := t.Context()
			service.EXPECT().Finish(ctx, trip.ID).Return(trip, test.err)
			handler := finish.NewHandler(service)
			router := gin.New()
			router.POST("/api/v1/trips/:trip_id/finish", func(c *gin.Context) { handler.FinishTrip(c, trip.ID) })
			recorder := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/trips/"+trip.ID.String()+"/finish", nil)
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if test.err != nil {
				var problem api.Problem
				if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
					t.Fatal(err)
				}
				if problem.Code != test.code || recorder.Header().Get("Content-Type") != "application/problem+json" || strings.Contains(recorder.Body.String(), "secret") {
					t.Fatalf("unexpected problem: %s", recorder.Body.String())
				}
				return
			}
			var result api.Trip
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Id != trip.ID || result.UserId != trip.UserID || result.DriverId != trip.DriverID || result.Price != trip.Price || result.FinishedAt == nil || !result.FinishedAt.Equal(finished) || result.Status != api.TripStatus(trip.Status) {
				t.Fatalf("unexpected trip: %+v", result)
			}
		})
	}
}
