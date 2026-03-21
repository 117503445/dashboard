package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"

	"github.com/117503445/dashboard/internal/buildinfo"
	"github.com/117503445/dashboard/pkg/rpc"
	"github.com/117503445/dashboard/pkg/rpc/rpcconnect"
)

func init() {
	glog.InitZeroLog()
}

func main() {
	log.Info().
		Str("BuildTime", buildinfo.BuildTime).
		Str("GitBranch", buildinfo.GitBranch).
		Str("GitCommit", buildinfo.GitCommit).
		Str("GitTag", buildinfo.GitTag).
		Str("GitDirty", buildinfo.GitDirty).
		Str("GitVersion", buildinfo.GitVersion).
		Str("BuildDir", buildinfo.BuildDir).
		Msg("构建信息")

	port := os.Getenv("DASHBOARD_PORT")
	if port == "" {
		port = "8080"
	}

	// 读取 sshole-hub 配置
	hubURL := os.Getenv("DASHBOARD_SSHOLE_HUB_URL")
	hubToken := os.Getenv("DASHBOARD_SSHOLE_HUB_TOKEN")
	mockAgents := os.Getenv("DASHBOARD_MOCK_AGENTS")

	sshUser := os.Getenv("DASHBOARD_SSH_USER")
	if sshUser == "" {
		sshUser = "root"
	}
	sshPassword := os.Getenv("DASHBOARD_SSH_PASSWORD")
	sshKeyPath := os.Getenv("DASHBOARD_SSH_KEY_PATH")

	config := Config{
		HubURL:      hubURL,
		HubToken:    hubToken,
		MockAgents:  mockAgents,
		SSHUser:     sshUser,
		SSHPassword: sshPassword,
		SSHKeyPath:  sshKeyPath,
	}

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)

	go func() {
		time.Sleep(1 * time.Second)
		client := rpcconnect.NewTemplateServiceClient(http.DefaultClient, "http://localhost:"+port)
		resp, err := client.Healthz(context.Background(), connect.NewRequest(&rpc.HealthzRequest{}))
		if err != nil {
			log.Panic().Err(err).Msg("健康检查调用失败")
		}
		log.Info().
			Interface("resp", resp).
			Interface("header", resp.Header()).
			Msg("健康检查响应")
	}()

	if err := ListenAndServe(ctx, port, config); err != nil {
		log.Panic().Err(err).Msg("服务启动失败")
	}
}
