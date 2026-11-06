package middlewares

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const httpStatusBadRequest = 400

type Tracing struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

func NewTracing(tracer trace.Tracer, propagator propagation.TextMapPropagator) *Tracing {
	return &Tracing{tracer: tracer, propagator: propagator}
}

func (t *Tracing) Middleware(c *gin.Context) {
	ctx := c.Request.Context()
	ctx = t.propagator.Extract(ctx, propagation.HeaderCarrier(c.Request.Header))

	route := c.FullPath()
	if route == "" {
		route = "unknown"
	}

	ctx, span := t.tracer.Start(ctx, c.Request.Method+" "+route,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("http.route", route),
		),
	)

	c.Request = c.Request.WithContext(ctx)
	defer func() {
		value := recover()
		if value != nil {
			c.Status(http.StatusInternalServerError)
			span.RecordError(fmt.Errorf("panic: %v", value))
		}
		finishSpan(c, span)
		span.End()
		if value != nil {
			panic(value)
		}
	}()
	c.Next()
}

func finishSpan(c *gin.Context, span trace.Span) {
	statusCode := c.Writer.Status()
	span.SetAttributes(
		attribute.Int("http.status_code", statusCode),
		attribute.Int("http.response.status_code", statusCode),
		attribute.String("http.status_text", strconv.Itoa(statusCode)),
	)

	if statusCode >= httpStatusBadRequest {
		span.SetAttributes(attribute.String("error.type", strconv.Itoa(statusCode)))
		if statusCode >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(statusCode))
		}
		for _, err := range c.Errors {
			span.RecordError(err.Err)
		}
	}
}
