package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/RhinoLink-Tech/data-fabric/internal/config"
)

func TestNewHTTPServerAppliesConfiguredTimeouts(t *testing.T) {
	cfg := config.ServerConfig{
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       7 * time.Second,
		WriteTimeout:      11 * time.Second,
		IdleTimeout:       13 * time.Second,
	}
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newHTTPServer(":8080", handler, cfg)

	if server.Addr != ":8080" || server.Handler == nil {
		t.Fatalf("server address/handler not configured: %+v", server)
	}
	if server.ReadHeaderTimeout != cfg.ReadHeaderTimeout ||
		server.ReadTimeout != cfg.ReadTimeout ||
		server.WriteTimeout != cfg.WriteTimeout ||
		server.IdleTimeout != cfg.IdleTimeout {
		t.Fatalf("server timeouts = (%s, %s, %s, %s), want (%s, %s, %s, %s)",
			server.ReadHeaderTimeout, server.ReadTimeout, server.WriteTimeout, server.IdleTimeout,
			cfg.ReadHeaderTimeout, cfg.ReadTimeout, cfg.WriteTimeout, cfg.IdleTimeout)
	}
}
