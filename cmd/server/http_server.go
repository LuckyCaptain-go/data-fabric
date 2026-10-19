package main

import (
	"net/http"

	"github.com/RhinoLink-Tech/data-fabric/internal/config"
)

func newHTTPServer(addr string, handler http.Handler, cfg config.ServerConfig) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}
