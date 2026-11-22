package positions

import (
	"net/http"

	"github.com/imrtlDrifter/template/internal/domain/models"
	api "github.com/imrtlDrifter/template/internal/generated"
	"github.com/imrtlDrifter/template/internal/handlers/v1/domainerror"
	httpres "github.com/imrtlDrifter/template/pkg/http_res"
	"github.com/imrtlDrifter/template/pkg/utils"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	tripService tripService
}

func NewHandler(tripService tripService) *Handler {
	return &Handler{tripService: tripService}
}

func (h *Handler) CreateTripPosition(c *gin.Context, tripID api.TripId) {
	var req api.CreateTripPositionJSONRequestBody
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		httpres.NewError(c, http.StatusBadRequest, err)
		return
	}

	body := c.MustGet(gin.BodyBytesKey).([]byte)
	if err := utils.ValidatePositionRequiredFields(body); err != nil {
		httpres.HandleDomainError(c, domainerror.WrapValidationError(err))
		return
	}

	position := &models.TripPosition{
		TripID:      tripID,
		RecordedAt:  req.RecordedAt,
		Coordinates: models.Coordinates(req.Coordinates),
	}
	if err := h.tripService.Create(c.Request.Context(), position); err != nil {
		httpres.HandleDomainError(c, domainerror.WrapTripError(err))
		return
	}

	c.JSON(http.StatusCreated, positionResponse(*position))
}

func (h *Handler) ListTripPositions(c *gin.Context, tripID api.TripId) {
	positions, err := h.tripService.List(c.Request.Context(), tripID)
	if err != nil {
		httpres.HandleDomainError(c, domainerror.WrapTripError(err))
		return
	}

	result := make([]api.TripPosition, 0, len(positions))
	for _, position := range positions {
		result = append(result, positionResponse(position))
	}
	c.JSON(http.StatusOK, api.TripPositionsResponse{Positions: result})
}

func positionResponse(position models.TripPosition) api.TripPosition {
	return api.TripPosition{
		Id:          position.ID,
		TripId:      position.TripID,
		Coordinates: api.Coordinates(position.Coordinates),
		RecordedAt:  position.RecordedAt,
	}
}
