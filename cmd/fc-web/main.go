package main

import (
	"context"
	"os"

	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"
)

func init() {
	glog.InitZeroLog()
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)

	if err := ListenAndServe(ctx, port); err != nil {
		log.Panic().Err(err).Msg("failed to serve")
	}
}
