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
		Msg("build info")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)

	go func() {
		time.Sleep(1 * time.Second)
		client := rpcconnect.NewTemplateServiceClient(http.DefaultClient, "http://localhost:"+port)
		resp, err := client.Healthz(context.Background(), connect.NewRequest(&rpc.HealthzRequest{}))
		if err != nil {
			log.Panic().Err(err).Msg("failed to call healthz")
		}
		log.Info().
			Interface("resp", resp).
			Interface("header", resp.Header()).
			Msg("healthz response")
	}()

	if err := ListenAndServe(ctx, port); err != nil {
		log.Panic().Err(err).Msg("failed to serve")
	}
}
