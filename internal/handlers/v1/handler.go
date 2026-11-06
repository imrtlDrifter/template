package v1

import (
	api "github.com/imrtlDrifter/template/internal/generated"
	httpres "github.com/imrtlDrifter/template/pkg/http_res"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	db            readinessChecker
	createHandler createHandler
	getHandler    getHandler
	finishHandler finishHandler
}

var _ api.ServerInterface = (*Handler)(nil)

func NewHandler(db readinessChecker, createHandler createHandler, getHandler getHandler, finishHandler finishHandler) *Handler {
	return &Handler{
		db:            db,
		createHandler: createHandler,
		getHandler:    getHandler,
		finishHandler: finishHandler,
	}
}

func (h *Handler) CreateTrip(c *gin.Context, params api.CreateTripParams) {
	h.createHandler.CreateTrip(c, params)
}

func (h *Handler) GetTrip(c *gin.Context, tripID api.TripId) {
	h.getHandler.GetTrip(c, tripID)
}

func (h *Handler) FinishTrip(c *gin.Context, tripID api.TripId) {
	h.finishHandler.FinishTrip(c, tripID)
}

func (h *Handler) Init(router gin.IRouter) {
	api.RegisterHandlersWithOptions(router, h, api.GinServerOptions{
		ErrorHandler: func(c *gin.Context, err error, statusCode int) {
			httpres.NewError(c, statusCode, err)
		},
	})
}
