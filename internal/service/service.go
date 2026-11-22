package service

import (
	"errors"

	"github.com/imrtlDrifter/template/internal/service/trip/create"
	"github.com/imrtlDrifter/template/internal/service/trip/finish"
	"github.com/imrtlDrifter/template/internal/service/trip/get"
	"github.com/imrtlDrifter/template/internal/service/trip/positions"
)

type Service struct {
	TripCreate    *create.TripService
	TripGet       *get.TripService
	TripFinish    *finish.TripService
	TripPositions *positions.TripService
}

func NewService(tripCreate *create.TripService, tripGet *get.TripService, tripFinish *finish.TripService, tripPositions *positions.TripService) (*Service, error) {
	if tripCreate == nil || tripGet == nil || tripFinish == nil || tripPositions == nil {
		return nil, errors.New("trip service is nil")
	}

	return &Service{
		TripCreate:    tripCreate,
		TripGet:       tripGet,
		TripFinish:    tripFinish,
		TripPositions: tripPositions,
	}, nil
}
