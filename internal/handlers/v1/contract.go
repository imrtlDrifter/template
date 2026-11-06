package v1

import (
	"context"

	api "github.com/imrtlDrifter/template/internal/generated"

	"github.com/gin-gonic/gin"
)

//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type (
	createHandler interface {
		CreateTrip(c *gin.Context, params api.CreateTripParams)
	}

	getHandler interface {
		GetTrip(c *gin.Context, tripID api.TripId)
	}

	finishHandler interface {
		FinishTrip(c *gin.Context, tripID api.TripId)
	}

	readinessChecker interface {
		CheckReady(ctx context.Context) error
	}
)
