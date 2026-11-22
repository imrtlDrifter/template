package models

import (
	"time"

	"github.com/imrtlDrifter/template/internal/domainerrors"

	"github.com/google/uuid"
)

type TripPosition struct {
	RecordedAt  time.Time
	TripID      uuid.UUID
	Coordinates Coordinates
	ID          int64
}

type StaleTrip struct {
	Since    time.Time
	ID       uuid.UUID
	DriverID uuid.UUID
}

func (p *TripPosition) Validate(now time.Time) error {
	switch {
	case !(p.Coordinates.Latitude >= -90 && p.Coordinates.Latitude <= 90):
		return &domainerrors.ValidationError{Field: "coordinates.latitude", Message: "must be between -90 and 90"}
	case !(p.Coordinates.Longitude >= -180 && p.Coordinates.Longitude <= 180):
		return &domainerrors.ValidationError{Field: "coordinates.longitude", Message: "must be between -180 and 180"}
	case p.RecordedAt.After(now.Add(time.Minute)):
		return &domainerrors.ValidationError{Field: "recorded_at", Message: "must not be more than one minute in the future"}
	}
	return nil
}
