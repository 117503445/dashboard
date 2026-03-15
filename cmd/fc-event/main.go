package main

import (
	"context"
	"fmt"

	"github.com/117503445/goutils/glog"
	"github.com/aliyun/fc-runtime-go-sdk/fc"
	"github.com/aliyun/fc-runtime-go-sdk/fccontext"
	"github.com/rs/zerolog/log"

	"github.com/117503445/dashboard/internal/buildinfo"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func handle(ctx context.Context, event any) Response {
	log.Ctx(ctx).Info().
		Interface("event", event).
		Msg("handle")
	return Response{
		Code:    0,
		Message: "success",
		Data:    nil,
	}
}

func HandleRequest(ctx context.Context, event any) (resp any, err error) {
	glog.InitZeroLog()

	fcCtx, success := fccontext.FromContext(ctx)
	if success {
		requestID := fcCtx.RequestID
		log.Logger = log.Output(glog.NewConsoleWriter(
			glog.ConsoleWriterConfig{
				RequestId: requestID,
				DirBuild:  buildinfo.BuildDir,
			},
		))
		ctx = WithContext(ctx, AppContext{
			RequestID: requestID,
			FcCtx:     fcCtx,
		})
	}

	ctx = log.Logger.WithContext(ctx)

	defer func() {
		if r := recover(); r != nil {
			log.Info().Ctx(ctx).Interface("r", r).Msg("recover")
			resp = ""
			err = fmt.Errorf("%v", r)
		}
	}()

	handle(ctx, event)

	return "success", nil
}

func main() {
	fc.Start(HandleRequest)
}
