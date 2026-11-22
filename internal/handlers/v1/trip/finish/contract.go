package finish

import (
	"context"

	"github.com/imrtlDrifter/template/internal/domain/models"

	"github.com/google/uuid"
)

//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type tripService interface {
	Finish(ctx context.Context, id uuid.UUID) (*models.Trip, error)
}
