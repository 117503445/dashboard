package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"

	"github.com/117503445/goutils/gclient/aliyun"
	fc20230330 "github.com/alibabacloud-go/fc-20230330/v4/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/rs/zerolog/log"
)

func invokeFC() {
	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	logger := log.Ctx(ctx)

	logger.Info().
		Str("service", cli.InvokeFC.ServiceName).
		Str("function", cli.InvokeFC.FunctionName).
		Str("payload", cli.InvokeFC.Payload).
		Msg("Invoking FC function")

	// Create FC client for authentication
	fcClient, err := aliyun.NewFc3Client(ctx, aliyun.Fc3ClientParams{
		Region:          "cn-hangzhou",
		AccountID:       cli.AccountID,
		AccessKeyId:     cli.AccessKeyID,
		AccessKeySecret: cli.AccessKeySecret,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create FC client")
	}

	result, err := fcClient.InvokeFunctionWithOptions(tea.String(cli.InvokeFC.FunctionName), &fc20230330.InvokeFunctionRequest{
		Body: bytes.NewReader([]byte("{}")),
	},
		&fc20230330.InvokeFunctionHeaders{
			XFcLogType: tea.String("Tail"),
		},
		&dara.RuntimeOptions{})

	if err != nil {
		logger.Panic().Err(err).Msg("invoke failed")
	}
	logger.Info().Interface("result", result).Send()
	logBytes, err := base64.StdEncoding.DecodeString(tea.StringValue(result.Headers["x-fc-log-result"]))
	if err != nil {
		logger.Panic().Err(err).Msg("failed to decode log content")
	}
	logContent := string(logBytes)
	fmt.Println(logContent)
	// logger.Info().Str("logContent", logContent).Msg("log content")
}
