package create

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/imrtlDrifter/template/internal/domainerrors"
	api "github.com/imrtlDrifter/template/internal/generated"
	"github.com/imrtlDrifter/template/internal/handlers/v1/domainerror"
	triphandler "github.com/imrtlDrifter/template/internal/handlers/v1/trip"
	httpres "github.com/imrtlDrifter/template/pkg/http_res"
	"github.com/imrtlDrifter/template/pkg/utils"

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

func (h *Handler) CreateTrip(c *gin.Context, params api.CreateTripParams) {
	var req api.CreateTripJSONRequestBody
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		if errors.Is(err, io.EOF) {
			httpres.HandleDomainError(c, domainerror.WrapTripError(domainerrors.ErrEmptyRequestBody))
			return
		}
		httpres.NewError(c, http.StatusBadRequest, err)
		return
	}

	body := c.MustGet(gin.BodyBytesKey).([]byte)
	if err := utils.ValidateCreateTripRequiredFields(body); err != nil {
		httpres.HandleDomainError(c, domainerror.WrapValidationError(err))
		return
	}

	ctx := c.Request.Context()
	trip, created, err := h.tripService.Create(ctx, CreateTrip(req), params.IdempotencyKey)
	if err != nil {
		httpres.HandleDomainError(c, domainerror.WrapTripError(err))
		return
	}

	if created {
		c.Header("Location", fmt.Sprintf("%s/v1/trips/%s", utils.APIBasePath, trip.ID))
		c.JSON(http.StatusCreated, triphandler.TripResponse(trip))
		return
	}

	c.JSON(http.StatusOK, triphandler.TripResponse(trip))
}
