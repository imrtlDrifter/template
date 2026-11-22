package middlewares

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	RequestDuration metric.Float64Histogram
	ActiveRequests  metric.Int64UpDownCounter
}

func (metrics Metrics) Middleware(c *gin.Context) {
	start := time.Now()
	route := c.FullPath()
	if route == "" {
		route = "unknown"
	}
	attrs := []attribute.KeyValue{attribute.String("http.request.method", c.Request.Method), attribute.String("http.route", route)}
	ctx := c.Request.Context()
	metrics.ActiveRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
	defer func() {
		value := recover()
		if value != nil {
			c.Status(http.StatusInternalServerError)
		}
		metrics.ActiveRequests.Add(ctx, -1, metric.WithAttributes(attrs...))
		attrs = append(attrs, attribute.Int("http.response.status_code", c.Writer.Status()))
		metrics.RequestDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
		if value != nil {
			panic(value)
		}
	}()
	c.Next()
}
