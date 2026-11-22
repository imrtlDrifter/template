package models_test

import (
	"errors"
	"testing"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"

	"github.com/google/uuid"
)

func TestCreateTripCommandValidate(t *testing.T) {
	validCommand := models.CreateTripCommand{
		UserID:   uuid.New(),
		DriverID: uuid.New(),
		StartPoint: models.Coordinates{
			Latitude:  59.9,
			Longitude: 30.3,
		},
		EndPoint: models.Coordinates{
			Latitude:  55.7,
			Longitude: 37.6,
		},
		Price: 1450,
	}

	tests := []struct {
		name   string
		change func(*models.CreateTripCommand)
		field  string
	}{
		{name: "valid"},
		{name: "empty user", change: func(command *models.CreateTripCommand) {
			command.UserID = uuid.Nil
		}, field: "user_id"},
		{name: "first error wins", change: func(command *models.CreateTripCommand) {
			command.UserID = uuid.Nil
			command.Price = -1
		}, field: "user_id"},
		{name: "empty driver", change: func(command *models.CreateTripCommand) {
			command.DriverID = uuid.Nil
		}, field: "driver_id"},
		{name: "start latitude too low", change: func(command *models.CreateTripCommand) {
			command.StartPoint.Latitude = -91
		}, field: "start_point.latitude"},
		{name: "start latitude too high", change: func(command *models.CreateTripCommand) {
			command.StartPoint.Latitude = 91
		}, field: "start_point.latitude"},
		{name: "start longitude too low", change: func(command *models.CreateTripCommand) {
			command.StartPoint.Longitude = -181
		}, field: "start_point.longitude"},
		{name: "start longitude too high", change: func(command *models.CreateTripCommand) {
			command.StartPoint.Longitude = 181
		}, field: "start_point.longitude"},
		{name: "end latitude too low", change: func(command *models.CreateTripCommand) {
			command.EndPoint.Latitude = -91
		}, field: "end_point.latitude"},
		{name: "end latitude too high", change: func(command *models.CreateTripCommand) {
			command.EndPoint.Latitude = 91
		}, field: "end_point.latitude"},
		{name: "end longitude too low", change: func(command *models.CreateTripCommand) {
			command.EndPoint.Longitude = -181
		}, field: "end_point.longitude"},
		{name: "end longitude too high", change: func(command *models.CreateTripCommand) {
			command.EndPoint.Longitude = 181
		}, field: "end_point.longitude"},
		{name: "negative price", change: func(command *models.CreateTripCommand) {
			command.Price = -1
		}, field: "price"},
		{name: "lower boundaries and free trip", change: func(command *models.CreateTripCommand) {
			command.StartPoint = models.Coordinates{Latitude: -90, Longitude: -180}
			command.EndPoint = command.StartPoint
			command.Price = 0
		}},
		{name: "upper boundaries", change: func(command *models.CreateTripCommand) {
			command.StartPoint = models.Coordinates{Latitude: 90, Longitude: 180}
			command.EndPoint = command.StartPoint
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := validCommand
			if test.change != nil {
				test.change(&command)
			}

			err := command.Validate()
			if test.field == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}

			var validationErr *domainerrors.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("Validate() error = %v, expected ValidationError", err)
			}
			if validationErr.Field != test.field || validationErr.Message == "" {
				t.Fatalf("unexpected validation error: %+v", validationErr)
			}
		})
	}
}

func TestTripFinish(t *testing.T) {
	finishedAt := time.Date(2026, time.November, 6, 12, 0, 0, 0, time.UTC)
	trip := models.Trip{Status: models.TripStatusActive}

	if err := trip.Finish(finishedAt); err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if trip.Status != models.TripStatusCompleted || trip.FinishedAt == nil || !trip.FinishedAt.Equal(finishedAt) {
		t.Fatalf("unexpected completed trip: %+v", trip)
	}

	if err := trip.Finish(finishedAt.Add(time.Hour)); !errors.Is(err, domainerrors.ErrTripCompleted) {
		t.Fatalf("second Finish() error = %v, expected ErrTripCompleted", err)
	}
	if trip.Status != models.TripStatusCompleted || trip.FinishedAt == nil || !trip.FinishedAt.Equal(finishedAt) {
		t.Fatal("repeated Finish changed the trip")
	}
}
