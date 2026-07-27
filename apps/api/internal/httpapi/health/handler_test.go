package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lzyu/QuickEval/apps/api/internal/httpapi/requestid"
)

func TestReadyReturnsOKWhenDependenciesAreHealthy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(
		func(context.Context) error { return nil },
		func(context.Context) error { return nil },
	)
	router := gin.New()
	router.Use(requestid.Middleware())
	router.GET("/health/ready", handler.Ready)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s, want healthy status", recorder.Body.String())
	}
	if recorder.Header().Get(requestid.Header) == "" {
		t.Fatal("response is missing request ID")
	}
}

func TestReadyDoesNotExposeDependencyError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(
		func(context.Context) error { return errors.New("secret database address") },
		func(context.Context) error { return nil },
	)
	router := gin.New()
	router.Use(requestid.Middleware())
	router.GET("/health/ready", handler.Ready)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	if strings.Contains(recorder.Body.String(), "secret database address") {
		t.Fatalf("body leaked dependency error: %s", recorder.Body.String())
	}
}
