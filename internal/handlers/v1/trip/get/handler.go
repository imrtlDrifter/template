package get

import (
	"net/http"

	api "github.com/imrtlDrifter/template/internal/generated"
	"github.com/imrtlDrifter/template/internal/handlers/v1/domainerror"
	triphandler "github.com/imrtlDrifter/template/internal/handlers/v1/trip"
	httpres "github.com/imrtlDrifter/template/pkg/http_res"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	tripService tripService
}

func NewHandler(tripService tripService) *Handler {
	return &Handler{
		tripService: tripService,
	}
}

func (h *Handler) GetTrip(c *gin.Context, tripID api.TripId) {
	ctx := c.Request.Context()
	trip, err := h.tripService.GetByID(ctx, tripID)
	if err != nil {
		httpres.HandleDomainError(c, domainerror.WrapTripError(err))
		return
	}

	c.JSON(http.StatusOK, triphandler.TripResponse(trip))
}
