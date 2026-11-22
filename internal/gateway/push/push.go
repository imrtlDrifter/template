package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/imrtlDrifter/template/internal/domain/models"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrTransport = errors.New("push transport error")
	ErrClient    = errors.New("push client error")
	ErrServer    = errors.New("push server error")
)

type Gateway struct {
	client     *http.Client
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
	endpoint   string
}

type request struct {
	Data        map[string]string `json:"data"`
	RequestID   string            `json:"request_id"`
	Kind        string            `json:"kind"`
	RecipientID uuid.UUID         `json:"recipient_id"`
}

func NewGateway(baseURL string, timeout time.Duration, tracer trace.Tracer, propagator propagation.TextMapPropagator) (*Gateway, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("PUSH_HTTP_URL must be an HTTP URL without credentials, query or fragment")
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			IdleConnTimeout:     90 * time.Second,
			MaxIdleConnsPerHost: 10,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &Gateway{
		endpoint:   parsed.JoinPath("api/v1/push").String(),
		client:     client,
		tracer:     tracer,
		propagator: propagator,
	}, nil
}

func (g *Gateway) RequestPosition(ctx context.Context, trip models.StaleTrip) error {
	ctx, span := g.tracer.Start(ctx, "push.request_position", trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attribute.String("http.request.method", http.MethodPost)))
	defer span.End()

	if err := g.send(ctx, trip, span); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "push request failed")
		return err
	}
	return nil
}

func (g *Gateway) send(ctx context.Context, trip models.StaleTrip, span trace.Span) error {
	requestID := uuid.NewString()
	body, err := json.Marshal(request{
		RequestID:   requestID,
		RecipientID: trip.DriverID,
		Kind:        "REQUEST_POSITION",
		Data:        map[string]string{"trip_id": trip.ID.String()},
	})
	if err != nil {
		return fmt.Errorf("encode push request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", requestID)
	g.propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTransport, err)
	}
	defer resp.Body.Close()
	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	_, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))

	switch {
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: HTTP %d", ErrServer, resp.StatusCode)
	case resp.StatusCode >= 400:
		return fmt.Errorf("%w: HTTP %d", ErrClient, resp.StatusCode)
	case resp.StatusCode != http.StatusAccepted:
		return fmt.Errorf("unexpected push response: HTTP %d", resp.StatusCode)
	case readErr != nil:
		return fmt.Errorf("%w: read response: %w", ErrTransport, readErr)
	}
	return nil
}

func (g *Gateway) Close() {
	g.client.CloseIdleConnections()
}
