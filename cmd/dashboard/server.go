package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"

	"github.com/117503445/dashboard/pkg/rpc/rpcconnect"
)

func ListenAndServe(ctx context.Context, port string, config Config) error {
	var forwardManager *ForwardManager
	if config.HubURL != "" {
		sshConfig, err := buildSSHConfig(config)
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Msg("failed to build SSH config, SSH forwarding disabled")
		} else {
			forwardManager = NewForwardManager(ctx, config.HubURL, config.HubToken, sshConfig)
			defer forwardManager.Close()
			log.Ctx(ctx).Info().Str("hubURL", config.HubURL).Str("sshUser", config.SSHUser).Msg("SSH forwarding enabled")
		}
	}

	mux := http.NewServeMux()
	server := NewServer(config, forwardManager)

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

func buildSSHConfig(config Config) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	if config.SSHKeyPath != "" {
		key, err := os.ReadFile(config.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key %s: %w", config.SSHKeyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if config.SSHPassword != "" {
		authMethods = append(authMethods, ssh.Password(config.SSHPassword))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH auth method configured (set DASHBOARD_SSH_PASSWORD or DASHBOARD_SSH_KEY_PATH)")
	}

	return &ssh.ClientConfig{
		User:            config.SSHUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}, nil
}
