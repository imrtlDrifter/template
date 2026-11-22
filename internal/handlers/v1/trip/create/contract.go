package create

import (
	"context"

	"github.com/imrtlDrifter/template/internal/domain/models"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type tripService interface {
	Create(ctx context.Context, req *models.CreateTripCommand, idempotencyKey *uuid.UUID) (*models.Trip, bool, error)
}
