package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/117503445/goutils"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"
)

func release() {
	glog.InitZeroLog()

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	log.Ctx(ctx).Info().Msg("release build")

	// 创建输出目录
	releaseDir := "./data/release"
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		log.Ctx(ctx).Error().Err(err).Str("dir", releaseDir).Msg("failed to create release directory")
		os.Exit(1)
	}
	log.Ctx(ctx).Info().Str("dir", releaseDir).Msg("created release directory")

	// 获取构建信息
	buildInfo, err := goutils.GetBuildInfo(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to get build info")
		os.Exit(1)
	}

	// 定义目标平台和架构
	targets := []struct {
		os   string
		arch string
	}{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
		{"windows", "arm64"},
	}

	// 并行构建
	var wg sync.WaitGroup

	for _, target := range targets {
		for _, build := range []struct {
			name string
			path string
		}{
			{"cli", "./cmd/cli"},
		} {
			wg.Add(1)
			go func(target struct {
				os   string
				arch string
			}, build struct {
				name string
				path string
			}) {
				defer wg.Done()

				ctx := log.Output(glog.NewConsoleWriter(
					glog.ConsoleWriterConfig{
						RequestId: fmt.Sprintf("release-%s-%s-%s", build.name, target.os, target.arch),
					})).WithContext(ctx)

				log.Ctx(ctx).Info().Msg("building release binary")

				// 构建输出文件名
				ext := ""
				if target.os == "windows" {
					ext = ".exe"
				}
				outFile := fmt.Sprintf("./data/release/%s-%s-%s%s", build.name, target.os, target.arch, ext)

				ldflags := fmt.Sprintf(
					"-X 'github.com/117503445/dashboard/internal/buildinfo.BuildTime=%s' "+
						"-X 'github.com/117503445/dashboard/internal/buildinfo.GitBranch=%s' "+
						"-X 'github.com/117503445/dashboard/internal/buildinfo.GitCommit=%s' "+
						"-X 'github.com/117503445/dashboard/internal/buildinfo.GitTag=%s' "+
						"-X 'github.com/117503445/dashboard/internal/buildinfo.GitDirty=%t' "+
						"-X 'github.com/117503445/dashboard/internal/buildinfo.GitVersion=%s' "+
						"-X 'github.com/117503445/dashboard/internal/buildinfo.BuildDir=%s'",
					buildInfo.BuildTime, buildInfo.GitBranch, buildInfo.GitCommit,
					buildInfo.GitTag, buildInfo.GitDirty, buildInfo.GitVersion, buildInfo.BuildDir,
				)

				cmd := exec.Command("go", "build", "-o", outFile, "-ldflags", ldflags, "-trimpath", build.path)
				cmd.Dir = "../.."
				cmd.Env = os.Environ()
				cmd.Env = append(cmd.Env,
					fmt.Sprintf("GOOS=%s", target.os),
					fmt.Sprintf("GOARCH=%s", target.arch),
					"CGO_ENABLED=0",
				)

				if output, err := cmd.CombinedOutput(); err != nil {
					log.Ctx(ctx).Panic().Err(err).Str("output", string(output)).Msg("failed to build release binary")
					return
				}

				log.Ctx(ctx).Info().Str("output", outFile).Msg("built release binary successfully")
			}(target, build)
		}
	}

	wg.Wait()

	log.Ctx(ctx).Info().Msg("all release builds completed")
}
