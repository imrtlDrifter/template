package create_test

import (
	"context"
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
	"github.com/imrtlDrifter/template/internal/handlers/v1/trip/create"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestCreateTrip(t *testing.T) {
	trip := &models.Trip{ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(), Price: 100, Status: models.TripStatusActive, StartedAt: time.Now().UTC()}
	body := `{"user_id":"` + trip.UserID.String() + `","driver_id":"` + trip.DriverID.String() + `","start_point":{"latitude":0,"longitude":0},"end_point":{"latitude":1,"longitude":2},"price":100}`
	tests := []struct {
		name        string
		body        string
		serviceErr  error
		code        string
		status      int
		callService bool
		created     bool
	}{
		{name: "created", body: body, status: http.StatusCreated, callService: true, created: true},
		{name: "replayed", body: body, status: http.StatusOK, callService: true},
		{name: "busy", body: body, serviceErr: domainerrors.ErrDriverBusy, status: http.StatusConflict, code: "driver_busy", callService: true},
		{name: "validation", body: body, serviceErr: &domainerrors.ValidationError{Field: "price", Message: "invalid"}, status: http.StatusBadRequest, code: "invalid_request", callService: true},
		{name: "internal", body: body, serviceErr: errors.New("secret database password"), status: http.StatusInternalServerError, code: "internal_error", callService: true},
		{name: "empty", status: http.StatusBadRequest, code: "invalid_request"},
		{name: "malformed", body: `{`, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "missing", body: `{}`, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "null", body: `null`, status: http.StatusBadRequest, code: "invalid_request"},
		{name: "missing coordinate", body: strings.Replace(body, `"latitude":0,`, "", 1), status: http.StatusBadRequest, code: "invalid_request"},
		{name: "null price", body: strings.Replace(body, `"price":100`, `"price":null`, 1), status: http.StatusBadRequest, code: "invalid_request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewMocktripService(gomock.NewController(t))
			key := uuid.New()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if test.callService {
				service.EXPECT().Create(ctx, gomock.Any(), &key).DoAndReturn(func(received context.Context, req *models.CreateTripCommand, _ *uuid.UUID) (*models.Trip, bool, error) {
					if received != ctx || req.UserID != trip.UserID || req.DriverID != trip.DriverID || req.Price != trip.Price || req.StartPoint.Latitude != 0 || req.EndPoint.Longitude != 2 {
						t.Fatalf("unexpected command: %+v", req)
					}
					return trip, test.created, test.serviceErr
				})
			}
			handler := create.NewHandler(service)
			router := gin.New()
			router.POST("/api/v1/trips", func(c *gin.Context) { handler.CreateTrip(c, api.CreateTripParams{IdempotencyKey: &key}) })
			request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/trips", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			if test.code != "" {
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
			if result.Id != trip.ID || result.Price != trip.Price || result.Status != api.TripStatus(trip.Status) {
				t.Fatalf("unexpected trip: %+v", result)
			}
			location := recorder.Header().Get("Location")
			if test.created && location != "/api/v1/trips/"+trip.ID.String() || !test.created && location != "" {
				t.Fatalf("unexpected Location: %s", location)
			}
		})
	}
}
