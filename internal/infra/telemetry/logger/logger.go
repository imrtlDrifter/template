package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

const requestIDHeader = "X-Request-Id"

type Config struct {
	Service string
	Level   slog.Level
}

type Logger struct {
	log     *slog.Logger
	elastic *Elastic
}

type traceHandler struct {
	slog.Handler
}

func NewLogger(out io.Writer, cfg Config, elastic *Elastic) *Logger {
	handler := &traceHandler{Handler: slog.NewJSONHandler(out, &slog.HandlerOptions{Level: cfg.Level})}
	log := slog.New(handler).With("service", cfg.Service)
	if elastic != nil {
		elastic.log = log
	}
	return &Logger{log: log, elastic: elastic}
}

func (h *traceHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanContextFromContext(ctx)
	traceID, spanID := "", ""
	if span.IsValid() {
		traceID = span.TraceID().String()
		spanID = span.SpanID().String()
	}
	record.AddAttrs(slog.String("trace_id", traceID), slog.String("span_id", spanID))
	return h.Handler.Handle(ctx, record)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}

func (l *Logger) GinMiddleware(c *gin.Context) {
	start := time.Now()
	defer l.logRequest(c, start)
	c.Next()
}

func (l *Logger) logRequest(c *gin.Context, start time.Time) {
	route := c.FullPath()
	if route == "" {
		route = "unknown"
	}
	attrs := []any{"method", c.Request.Method, "route", route, "status", c.Writer.Status(), "duration_ms", time.Since(start).Milliseconds()}
	if rid := requestID(c); rid != "" {
		attrs = append(attrs, "request_id", rid)
	}
	if value, ok := c.Get("internal_error"); ok {
		attrs = append(attrs, "err", fmt.Sprint(value))
	}
	l.write(c.Request.Context(), levelByStatus(c.Writer.Status()), "request handled", attrs...)
}

func (l *Logger) InfoContext(ctx context.Context, msg string, extra ...any) {
	l.write(ctx, slog.LevelInfo, msg, extra...)
}

func (l *Logger) ErrorContext(ctx context.Context, err error, msg string, extra ...any) {
	if err == nil {
		return
	}
	l.write(ctx, slog.LevelError, msg, append([]any{"err", err.Error()}, extra...)...)
}

func (l *Logger) WarnContext(ctx context.Context, err error, msg string, extra ...any) {
	if err == nil {
		return
	}
	l.write(ctx, slog.LevelWarn, msg, append([]any{"err", err.Error()}, extra...)...)
}

func (l *Logger) ErrorCtx(c *gin.Context, err error, msg string, extra ...any) {
	l.ErrorContext(c.Request.Context(), err, msg, requestFields(c, extra)...)
}

func (l *Logger) InfoCtx(c *gin.Context, msg string, extra ...any) {
	l.InfoContext(c.Request.Context(), msg, requestFields(c, extra)...)
}

func (l *Logger) WarnCtx(c *gin.Context, err error, msg string, extra ...any) {
	l.WarnContext(c.Request.Context(), err, msg, requestFields(c, extra)...)
}

func (l *Logger) write(ctx context.Context, level slog.Level, msg string, attrs ...any) {
	l.log.Log(ctx, level, msg, attrs...)
	l.elastic.SendAppLog(ctx, level, msg, attrs...)
}

func (l *Logger) Shutdown(ctx context.Context) error {
	return l.elastic.Shutdown(ctx)
}

func WithRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if rid == "" {
			rid = strconv.FormatInt(time.Now().UnixNano(), 10)
		}
		c.Header(requestIDHeader, rid)
		c.Set("request_id", rid)
		c.Next()
	}
}

func requestID(c *gin.Context) string {
	if value, ok := c.Get("request_id"); ok {
		if id, ok := value.(string); ok {
			return id
		}
	}
	return strings.TrimSpace(c.GetHeader(requestIDHeader))
}

func requestFields(c *gin.Context, extra []any) []any {
	attrs := make([]any, 0, 6+len(extra))
	attrs = append(attrs, "method", c.Request.Method, "route", c.FullPath(), "request_id", requestID(c))
	return append(attrs, extra...)
}

func levelByStatus(code int) slog.Level {
	if code >= http.StatusInternalServerError {
		return slog.LevelError
	}
	return slog.LevelInfo
}
