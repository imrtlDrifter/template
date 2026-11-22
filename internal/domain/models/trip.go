package models

import (
	"time"

	"github.com/imrtlDrifter/template/internal/domainerrors"

	"github.com/google/uuid"
)

type TripStatus string

const (
	TripStatusActive    TripStatus = "active"
	TripStatusCompleted TripStatus = "completed"
)

type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type CreateTripCommand struct {
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint Coordinates
	EndPoint   Coordinates
	Price      int64
}

type Trip struct {
	StartedAt      time.Time   `json:"started_at"`
	FinishedAt     *time.Time  `json:"finished_at,omitempty"`
	LastPositionAt *time.Time  `json:"last_position_at,omitempty"`
	Status         TripStatus  `json:"status"`
	ID             uuid.UUID   `json:"id"`
	UserID         uuid.UUID   `json:"user_id"`
	DriverID       uuid.UUID   `json:"driver_id"`
	StartPoint     Coordinates `json:"start_point"`
	EndPoint       Coordinates `json:"end_point"`
	Price          int64       `json:"price"`
}

type TripStatusHistory struct {
	ChangedAt  time.Time   `json:"changed_at"`
	FromStatus *TripStatus `json:"from_status,omitempty"`
	Reason     *string     `json:"reason,omitempty"`
	ToStatus   TripStatus  `json:"to_status"`
	ID         int64       `json:"id"`
	TripID     uuid.UUID   `json:"trip_id"`
}

func (t *Trip) Finish(at time.Time) error {
	if t.Status != TripStatusActive {
		return domainerrors.ErrTripCompleted
	}

	t.Status = TripStatusCompleted
	t.FinishedAt = &at
	return nil
}
