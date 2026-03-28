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
	var forwardManager *ForwardManager
	if config.HubURL != "" {
		// 生成或加载 SSH 密钥对
		sshKeyPair, err := EnsureSSHKeyPair()
		if err != nil {
			log.Ctx(ctx).Err(err).Msg("SSH 密钥初始化失败，SSH 转发已禁用")
		} else {
			forwardManager = NewForwardManagerWithUser(ctx, config.HubURL, config.HubToken, config.SSHUser, sshKeyPair)
			defer forwardManager.Close()
			log.Ctx(ctx).Info().
				Str("hubURL", config.HubURL).
				Str("sshUser", forwardManager.sshConfig.User).
				Str("privateKey", sshKeyPair.KeyPath).
				Msg("SSH 转发已启用")
		}
	}

	mux := http.NewServeMux()
	server := NewServer(config, forwardManager)

	interceptors := connect.WithInterceptors(
		NewCtxInterceptor(),
	)

	// RPC 处理器（需在静态文件之前注册）
	path, handler := rpcconnect.NewTemplateServiceHandler(server, interceptors)
	mux.Handle(path, handler)

	// API 处理器（应用 HTTPMiddleware 注入 request ID）
	mux.Handle("/api/agents/", HTTPMiddleware(http.HandlerFunc(server.SetupCodeServerHandler)))

	// 代理处理器（应用 HTTPMiddleware 注入 request ID）
	mux.Handle("/proxy/agents/", HTTPMiddleware(http.HandlerFunc(server.ProxyHandler)))

	// 静态文件处理器（SPA 兜底路由，应用 HTTPMiddleware 注入 request ID）
	mux.Handle("/", HTTPMiddleware(staticHandler()))

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		MaxAge:         86400,
	})

	rootHandler := c.Handler(mux)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("监听端口失败")
		return err
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("关闭监听器失败")
		}
	}()

	log.Ctx(ctx).Info().Msgf("正在监听 %s", listener.Addr().String())
	log.Ctx(ctx).Info().Msgf("sshole-hub 地址: %s", config.HubURL)
	if err := http.Serve(listener, rootHandler); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("HTTP 服务异常退出")
		return err
	}
	return nil
}
