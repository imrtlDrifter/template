package v1_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "github.com/imrtlDrifter/template/internal/generated"
	v1 "github.com/imrtlDrifter/template/internal/handlers/v1"

	"github.com/gin-gonic/gin"
	"go.uber.org/mock/gomock"
)

func TestGeneratedRoutes(t *testing.T) {
	for _, test := range []struct {
		name   string
		method string
		path   string
		header string
	}{
		{name: "invalid path UUID", method: http.MethodGet, path: "/api/v1/trips/invalid"},
		{name: "invalid idempotency UUID", method: http.MethodPost, path: "/api/v1/trips", header: "invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			handler := v1.NewHandler(NewMockreadinessChecker(ctrl), NewMockcreateHandler(ctrl), NewMockgetHandler(ctrl), NewMockfinishHandler(ctrl))
			router := gin.New()
			handler.Init(router)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, nil)
			if test.header != "" {
				request.Header.Set("Idempotency-Key", test.header)
			}
			router.ServeHTTP(recorder, request)
			var problem api.Problem
			if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != http.StatusBadRequest || problem.Code != "invalid_request" {
				t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestHealthAndReady(t *testing.T) {
	for _, test := range []struct {
		err    error
		name   string
		path   string
		status int
	}{
		{name: "health", path: "/health", status: http.StatusOK},
		{name: "ready", path: "/ready", status: http.StatusOK},
		{name: "unavailable", path: "/ready", err: errors.New("secret DSN"), status: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := NewMockreadinessChecker(ctrl)
			if strings.HasSuffix(test.path, "/ready") {
				db.EXPECT().CheckReady(t.Context()).Return(test.err)
			}
			handler := v1.NewHandler(db, NewMockcreateHandler(ctrl), NewMockgetHandler(ctrl), NewMockfinishHandler(ctrl))
			router := gin.New()
			handler.Init(router)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, test.path, nil))
			if recorder.Code != test.status || strings.Contains(recorder.Body.String(), "secret") {
				t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
