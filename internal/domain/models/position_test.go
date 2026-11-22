package models_test

import (
	"errors"
	"testing"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"
	"github.com/imrtlDrifter/template/internal/domainerrors"
)

func TestPositionValidationBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, field         string
		latitude, longitude float64
		offset              time.Duration
	}{
		{name: "minimum", latitude: -90, longitude: -180},
		{name: "maximum", latitude: 90, longitude: 180, offset: time.Minute},
		{name: "latitude below minimum", field: "coordinates.latitude", latitude: -91},
		{name: "latitude above maximum", field: "coordinates.latitude", latitude: 91},
		{name: "longitude below minimum", field: "coordinates.longitude", longitude: -181},
		{name: "longitude above maximum", field: "coordinates.longitude", longitude: 181},
		{name: "future", field: "recorded_at", offset: time.Minute + time.Nanosecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			position := models.TripPosition{
				RecordedAt:  now.Add(test.offset),
				Coordinates: models.Coordinates{Latitude: test.latitude, Longitude: test.longitude},
			}
			err := position.Validate(now)
			if test.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var validationErr *domainerrors.ValidationError
			if !errors.As(err, &validationErr) || validationErr.Field != test.field {
				t.Fatalf("validate = %v, want validation error for %s", err, test.field)
			}
		})
	}
}
