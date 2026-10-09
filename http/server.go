package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hunterjob/hunterjob/api/http/controller"
)

type Server struct {
	HTTP *http.Server
}

func NewServer(port string, ctrl controller.Controller) *Server {
	router := gin.New()
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/v1/hunterjob/health", "/v1/hunterjob/ready"}}), gin.Recovery(), func(c *gin.Context) {
		c.Header("X-Request-ID", uuid.NewString())
		c.Next()
	})
	ctrl.Register(router)
	return &Server{HTTP: &http.Server{Addr: ":" + port, Handler: router, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}}
}

func (s *Server) Run(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_ = s.HTTP.Shutdown(shutdown)
			cancel()
		case <-done:
		}
	}()
	err := s.HTTP.ListenAndServe()
	close(done)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
