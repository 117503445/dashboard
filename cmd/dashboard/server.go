package main

import (
	"context"
	"net"
	"net/http"

	"connectrpc.com/connect"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"

	"github.com/117503445/dashboard/pkg/rpc/rpcconnect"
)

func ListenAndServe(ctx context.Context, port string, config Config) error {
	mux := http.NewServeMux()
	server := NewServer(config)

	interceptors := connect.WithInterceptors(
		NewCtxInterceptor(),
	)

	// RPC handler - must be registered before static files
	path, handler := rpcconnect.NewTemplateServiceHandler(server, interceptors)
	mux.Handle(path, handler)

	// Proxy handler for agent ports
	mux.Handle("/proxy/agents/", http.HandlerFunc(server.ProxyHandler))

	// Static files handler (catch-all for SPA)
	mux.Handle("/", staticHandler())

	// Enable CORS for all origins
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		MaxAge:         86400, // 1 day in seconds
	})

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to listen")
		return err
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("failed to close listener")
		}
	}()

	log.Ctx(ctx).Info().Msgf("listening on %s", listener.Addr().String())
	log.Ctx(ctx).Info().Msgf("sshole-hub URL: %s", config.HubURL)
	if err := http.Serve(listener, c.Handler(mux)); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to serve")
		return err
	}
	return nil
}
