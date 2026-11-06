package v1

import (
	"net/http"

	api "github.com/imrtlDrifter/template/internal/generated"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.db.CheckReady(ctx); err != nil {
		c.Set("internal_error", err.Error())
		c.JSON(http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}

	c.JSON(http.StatusOK, api.HealthResponse{Status: api.Ok})
}
