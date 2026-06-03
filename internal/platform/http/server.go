// Package http provides the optional echo-based observability server
// (healthz, metrics). It runs alongside the MCP stdio transport and is enabled
// via HTTP_ENABLED. It never serves on stdout (reserved for MCP stdio).
package http

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/tergeoo/asc-mcp/internal/shared/store"
)

// Server wraps an echo instance.
type Server struct {
	e    *echo.Echo
	addr string
}

// New builds the observability server. The store is used for a readiness check.
func New(addr string, st store.Store) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"status":   "ok",
			"dbActive": st.Enabled(),
		})
	})

	e.GET("/metrics", func(c echo.Context) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return c.JSON(http.StatusOK, map[string]any{
			"goroutines":    runtime.NumGoroutine(),
			"allocBytes":    m.Alloc,
			"sysBytes":      m.Sys,
			"numGC":         m.NumGC,
			"dbActive":      st.Enabled(),
		})
	})

	return &Server{e: e, addr: addr}
}

// Start runs the server until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.e.Shutdown(shutdownCtx)
	}()
	if err := s.e.Start(s.addr); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
