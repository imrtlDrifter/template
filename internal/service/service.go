package service

import (
	"errors"

	"github.com/imrtlDrifter/template/internal/service/trip/create"
	"github.com/imrtlDrifter/template/internal/service/trip/finish"
	"github.com/imrtlDrifter/template/internal/service/trip/get"
)

type Service struct {
	TripCreate *create.TripService
	TripGet    *get.TripService
	TripFinish *finish.TripService
}

func NewService(tripCreate *create.TripService, tripGet *get.TripService, tripFinish *finish.TripService) (*Service, error) {
	if tripCreate == nil || tripGet == nil || tripFinish == nil {
		return nil, errors.New("trip service is nil")
	}

	return &Service{
		TripCreate: tripCreate,
		TripGet:    tripGet,
		TripFinish: tripFinish,
	}, nil
}
