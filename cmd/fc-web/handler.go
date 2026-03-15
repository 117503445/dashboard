package main

import (
	"context"

	"connectrpc.com/connect"
	"github.com/117503445/goutils"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/117503445/dashboard/internal/buildinfo"
	"github.com/117503445/dashboard/pkg/rpc"
	"github.com/117503445/dashboard/pkg/rpc/rpcconnect"
)

func NewCtxInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(
			ctx context.Context,
			req connect.AnyRequest,
		) (resp connect.AnyResponse, err error) {
			requestID := ""
			if !req.Spec().IsClient {
				requestID = req.Header().Get("X-Request-ID")
				if requestID == "" {
					requestID = req.Header().Get("x-fc-request-id")
					if requestID == "" {
						requestID = goutils.UUID7()
					}
				}
				ctx = WithContext(ctx, AppContext{
					RequestID: requestID,
				})

				ctx = log.Output(glog.NewConsoleWriter(
					glog.ConsoleWriterConfig{
						RequestId: requestID,
						DirBuild:  buildinfo.BuildDir,
					},
				)).Level(zerolog.DebugLevel).With().Caller().Logger().WithContext(ctx)
				log.Ctx(ctx).Info().
					Str("BuildTime", buildinfo.BuildTime).
					Str("GitBranch", buildinfo.GitBranch).
					Str("GitCommit", buildinfo.GitCommit).
					Str("GitTag", buildinfo.GitTag).
					Str("GitDirty", buildinfo.GitDirty).
					Str("GitVersion", buildinfo.GitVersion).
					Str("BuildDir", buildinfo.BuildDir).
					Msg("build info")
				log.Ctx(ctx).Info().
					Interface("req", req).
					Msg("request received")
			}
			resp, err = next(ctx, req)
			if err != nil {
				return nil, err
			}
			if resp != nil && resp.Header() != nil {
				resp.Header().Set("X-Request-ID", requestID)
			}
			log.Ctx(ctx).Info().
				Interface("resp", resp).
				Msg("request done")
			return resp, err
		}
	}
}

func NewServer() *Server {
	return &Server{}
}

type Server struct {
}

func (s *Server) Healthz(ctx context.Context, req *connect.Request[rpc.HealthzRequest]) (*connect.Response[rpc.ApiResponse], error) {
	log.Ctx(ctx).Info().Msg("Healthz")
	return &connect.Response[rpc.ApiResponse]{
		Msg: &rpc.ApiResponse{
			Code:    0,
			Message: "success",
			Payload: &rpc.ApiResponse_Healthz{
				Healthz: &rpc.HealthzResponse{
					Version: buildinfo.GitVersion,
				},
			},
		},
	}, nil
}

// Compile-time assertion that Server implements SyncModelscopeServiceHandler
var _ rpcconnect.TemplateServiceHandler = (*Server)(nil)
