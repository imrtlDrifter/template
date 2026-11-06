package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/imrtlDrifter/template/internal/config"

	"go.opentelemetry.io/otel/trace"
)

type elasticState struct {
	client  *http.Client
	index   string
	service string
	host    string
	baseURL string
	login   string
	pass    string
}

const (
	maximum            = 2048
	maxElasticInFlight = 64
)

type Elastic struct {
	log     *slog.Logger
	slots   chan struct{}
	pending map[context.Context]context.CancelFunc
	state   elasticState
	timeout time.Duration
	mu      sync.Mutex
	wg      sync.WaitGroup
	closed  bool
}

func NewElastic(ctx context.Context, cfg config.Telemetry) (*Elastic, error) {
	if cfg.ElasticEndpoint == "" {
		return nil, nil
	}
	parsed, err := url.Parse(cfg.ElasticEndpoint)
	if err != nil {
		return nil, fmt.Errorf("parse elastic URL: %w", err)
	}
	client := &http.Client{Timeout: cfg.ElasticTimeout}
	initCtx, cancel := context.WithTimeout(ctx, cfg.ElasticTimeout)
	defer cancel()
	if initErr := ensureIndex(initCtx, client, parsed.String(), cfg.ElasticIndex, cfg.ElasticLogin, cfg.ElasticPassword); initErr != nil {
		client.CloseIdleConnections()
		return nil, fmt.Errorf("init elastic index: %w", initErr)
	}
	host, _ := os.Hostname()
	return &Elastic{state: elasticState{client: client, index: cfg.ElasticIndex, service: cfg.ServiceName, host: host, baseURL: strings.TrimRight(parsed.String(), "/"), login: cfg.ElasticLogin, pass: cfg.ElasticPassword}, pending: make(map[context.Context]context.CancelFunc), slots: make(chan struct{}, maxElasticInFlight), timeout: cfg.ElasticTimeout}, nil
}

func (e *Elastic) SendAppLog(ctx context.Context, level slog.Level, message string, attrs ...any) {
	if e == nil || !e.log.Enabled(ctx, level) {
		return
	}
	fields := attrsToMap(attrs...)
	doc := map[string]any{"@timestamp": time.Now().UTC().Format(time.RFC3339Nano), "level": mapLogLvl(level), "message": message, "service": e.state.service, "host": e.state.host}
	maps.Copy(doc, fields)
	span := trace.SpanContextFromContext(ctx)
	doc["trace_id"] = ""
	doc["span_id"] = ""
	if span.IsValid() {
		doc["trace_id"] = span.TraceID().String()
		doc["span_id"] = span.SpanID().String()
	}
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	select {
	case e.slots <- struct{}{}:
		e.wg.Add(1)
	default:
		e.mu.Unlock()
		return
	}
	sendCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.timeout)
	e.pending[sendCtx] = cancel
	e.mu.Unlock()
	go func(sendCtx context.Context, cancel context.CancelFunc) {
		defer func() {
			e.mu.Lock()
			delete(e.pending, sendCtx)
			e.mu.Unlock()
			cancel()
			<-e.slots
			e.wg.Done()
		}()
		if err := indexDocument(sendCtx, e.state, doc); err != nil {
			e.log.WarnContext(sendCtx, "elastic log delivery failed", "err", err.Error())
		}
	}(sendCtx, cancel)
}

func (e *Elastic) Shutdown(ctx context.Context) error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	done := make(chan struct{})
	go func() { e.wg.Wait(); close(done) }()
	defer e.state.client.CloseIdleConnections()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		e.mu.Lock()
		for _, cancel := range e.pending {
			cancel()
		}
		e.mu.Unlock()
		<-done
		return ctx.Err()
	}
}

func indexDocument(ctx context.Context, st elasticState, doc map[string]any) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal doc: %w", err)
	}

	endpoint := st.baseURL + "/" + url.PathEscape(st.index) + "/_doc"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if st.login != "" {
		req.SetBasicAuth(st.login, st.pass)
	}

	resp, err := st.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, readBodySnippet(resp.Body))
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func ensureIndex(ctx context.Context, client *http.Client, baseURL, index, login, pass string) error {
	headURL := baseURL + "/" + url.PathEscape(index)

	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, headURL, nil)
	if err != nil {
		return fmt.Errorf("build head request: %w", err)
	}
	if strings.TrimSpace(login) != "" {
		headReq.SetBasicAuth(login, pass)
	}

	headResp, err := client.Do(headReq)
	if err != nil {
		return fmt.Errorf("head index: %w", err)
	}
	headResp.Body.Close()

	if headResp.StatusCode == http.StatusOK {
		return nil
	}
	if headResp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("head index status %d", headResp.StatusCode)
	}

	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, headURL, nil)
	if err != nil {
		return fmt.Errorf("build put request: %w", err)
	}
	if strings.TrimSpace(login) != "" {
		putReq.SetBasicAuth(login, pass)
	}

	putResp, err := client.Do(putReq)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		responseBody := readBodySnippet(putResp.Body)
		if putResp.StatusCode == http.StatusBadRequest && strings.Contains(responseBody, "resource_already_exists_exception") {
			return nil
		}

		return fmt.Errorf("create index status %d: %s", putResp.StatusCode, responseBody)
	}
	_, _ = io.Copy(io.Discard, putResp.Body)
	return nil
}

func readBodySnippet(r io.Reader) string {
	b, err := io.ReadAll(io.LimitReader(r, maximum))
	if err != nil {
		return "failed to read response body"
	}
	return strings.TrimSpace(string(b))
}

func attrsToMap(attrs ...any) map[string]any {
	out := make(map[string]any, len(attrs)/2)
	for i := 0; i+1 < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		out[key] = attrs[i+1]
	}
	return out
}

func mapLogLvl(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARN"
	case level <= slog.LevelDebug:
		return "DEBUG"
	default:
		return "INFO"
	}
}
