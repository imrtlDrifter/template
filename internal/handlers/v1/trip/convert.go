package trip

import (
	"github.com/imrtlDrifter/template/internal/domain/models"
	api "github.com/imrtlDrifter/template/internal/generated"
)

func TripResponse(trip *models.Trip) api.Trip {
	return api.Trip{
		Id:       trip.ID,
		UserId:   trip.UserID,
		DriverId: trip.DriverID,
		StartPoint: api.Coordinates{
			Latitude:  trip.StartPoint.Latitude,
			Longitude: trip.StartPoint.Longitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  trip.EndPoint.Latitude,
			Longitude: trip.EndPoint.Longitude,
		},
		Price:          trip.Price,
		Status:         api.TripStatus(trip.Status),
		StartedAt:      trip.StartedAt,
		FinishedAt:     trip.FinishedAt,
		LastPositionAt: trip.LastPositionAt,
	}
}
