package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hunterjob/hunterjob/api/internal/agent"
)

func TestHealthAndReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name  string
		ready func(context.Context) error
		want  int
	}{
		{"ready", func(context.Context) error { return nil }, http.StatusOK},
		{"dependency unavailable", func(context.Context) error { return errors.New("offline") }, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			Controller{Telegram: &agent.TelegramProvider{}, Ready: tc.ready}.Register(router)
			for _, check := range []struct {
				path string
				want int
			}{{"/v1/hunterjob/health", http.StatusOK}, {"/v1/hunterjob/ready", tc.want}, {"/v1/hunterjob/dashboard", http.StatusServiceUnavailable}} {
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, check.path, nil))
				if w.Code != check.want {
					t.Errorf("%s: got %d, want %d", check.path, w.Code, check.want)
				}
			}
		})
	}
}
