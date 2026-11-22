package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	httpres "github.com/imrtlDrifter/template/pkg/http_res"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Recovery struct{ logger errorLogger }

func NewRecovery(logger errorLogger) *Recovery { return &Recovery{logger: logger} }

func (r *Recovery) Middleware(c *gin.Context) {
	defer recoverRequest(c.Request.Context(), c, r.logger)
	c.Next()
}

func recoverRequest(ctx context.Context, c *gin.Context, logger errorLogger) {
	if value := recover(); value != nil {
		err := fmt.Errorf("panic: %v", value)
		span := trace.SpanFromContext(ctx)
		span.RecordError(err)
		span.SetStatus(codes.Error, "internal_error")
		span.SetAttributes(attribute.String("error.type", "panic"))
		logger.ErrorContext(ctx, err, "request panic", "stack", string(debug.Stack()))
		httpres.NewError(c, http.StatusInternalServerError, err)
	}
}
