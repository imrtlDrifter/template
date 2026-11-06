package models

import (
	"time"

	"github.com/google/uuid"
)

type IdempotencyRecord struct {
	ExpiresAt   time.Time
	TripID      *uuid.UUID
	RequestHash []byte
	Key         uuid.UUID
}
