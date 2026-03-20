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
			log.Ctx(ctx).Warn().Err(err).Msg("SSH 配置构建失败，SSH 转发已禁用")
		} else {
			forwardManager = NewForwardManager(ctx, config.HubURL, config.HubToken, sshConfig)
			defer forwardManager.Close()
			log.Ctx(ctx).Info().Str("hubURL", config.HubURL).Str("sshUser", config.SSHUser).Msg("SSH 转发已启用")
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

	// API 处理器
	mux.Handle("/api/agents/", http.HandlerFunc(server.SetupCodeServerHandler))

	// 代理处理器
	mux.Handle("/proxy/agents/", http.HandlerFunc(server.ProxyHandler))

	// 静态文件处理器（SPA 兜底路由）
	mux.Handle("/", staticHandler())

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

func buildSSHConfig(config Config) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	if config.SSHKeyPath != "" {
		key, err := os.ReadFile(config.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("读取 SSH 密钥 %s 失败: %w", config.SSHKeyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("解析 SSH 密钥失败: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if config.SSHPassword != "" {
		authMethods = append(authMethods, ssh.Password(config.SSHPassword))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("未配置 SSH 认证方式（请设置 DASHBOARD_SSH_PASSWORD 或 DASHBOARD_SSH_KEY_PATH）")
	}

	return &ssh.ClientConfig{
		User:            config.SSHUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}, nil
}
