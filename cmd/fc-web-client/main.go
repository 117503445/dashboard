package main

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"

	"github.com/117503445/dashboard/pkg/rpc"
	"github.com/117503445/dashboard/pkg/rpc/rpcconnect"
)

func init() {
	glog.InitZeroLog()
}

func main() {
	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)

	client := rpcconnect.NewTemplateServiceClient(http.DefaultClient, "https://fc-web-bctarkwgly.cn-hangzhou.fcapp.run")
	request := connect.NewRequest(&rpc.HealthzRequest{})
	// request.Header().Set("x-fc-request-id", goutils.UUID7())
	resp, err := client.Healthz(ctx, request)
	if err != nil {
		log.Panic().Err(err).Msg("failed to call healthz")
	}
	log.Info().Interface("resp", resp).Interface("header", resp.Header()).Msg("healthz response")
}
