package handlers

import "github.com/gin-gonic/gin"

type Handler struct {
	handlerV1   handlerV1
	middlewares []gin.HandlerFunc
}

func NewHandler(handlerV1 handlerV1, middlewares ...gin.HandlerFunc) *Handler {
	return &Handler{
		handlerV1:   handlerV1,
		middlewares: middlewares,
	}
}

func (h *Handler) Init() *gin.Engine {
	gin.DisableConsoleColor()
	gin.EnableJsonDecoderDisallowUnknownFields()
	router := gin.New()
	router.HandleMethodNotAllowed = true

	router.Use(h.middlewares...)

	h.initAPI(router)

	return router
}

func (h *Handler) initAPI(router *gin.Engine) {
	h.handlerV1.Init(router)
}
