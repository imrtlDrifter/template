package create

import (
	"github.com/imrtlDrifter/template/internal/domain/models"
	api "github.com/imrtlDrifter/template/internal/generated"
)

func CreateTrip(req api.CreateTripJSONRequestBody) *models.CreateTripCommand {
	return &models.CreateTripCommand{
		UserID:   req.UserId,
		DriverID: req.DriverId,
		StartPoint: models.Coordinates{
			Latitude:  req.StartPoint.Latitude,
			Longitude: req.StartPoint.Longitude,
		},
		EndPoint: models.Coordinates{
			Latitude:  req.EndPoint.Latitude,
			Longitude: req.EndPoint.Longitude,
		},
		Price: req.Price,
	}
}
