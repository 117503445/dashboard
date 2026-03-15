package main

import (
	"context"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/117503445/goutils"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"
)

// pushImage 推送单个镜像
func pushImage(ctx context.Context, imageName, registry string) error {
	log.Ctx(ctx).Info().Str("image", imageName).Str("registry", registry).Msg("pushing docker image")

	cmd := exec.Command("docker", "push", imageName)
	cmd.Dir = "../.."
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("output", string(output)).Str("image", imageName).Str("registry", registry).Msg("failed to push docker image")
		return err
	}

	log.Ctx(ctx).Info().Str("image", imageName).Str("registry", registry).Msg("pushed docker image successfully")
	return nil
}

func buildDocker() {
	glog.InitZeroLog()

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	log.Ctx(ctx).Info().Msg("build docker")

	// 获取构建信息
	buildInfo, err := goutils.GetBuildInfo(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to get build info")
		os.Exit(1)
	}

	// 构造 tag
	gitCommit := buildInfo.GitCommit
	if len(gitCommit) > 7 {
		gitCommit = gitCommit[:7]
	}

	tag := "117503445/go-template-rpc:" + gitCommit
	aliyunTag := "registry.cn-hangzhou.aliyuncs.com/117503445/go-template-rpc:" + gitCommit

	// 如果是 dirty build，添加构建日期
	if buildInfo.GitDirty {
		buildDate := time.Now().Format("20060102-150405")
		tag += "-" + buildDate
		aliyunTag += "-" + buildDate
	}

	log.Ctx(ctx).Info().Str("tag", tag).Str("aliyunTag", aliyunTag).Bool("dirty", buildInfo.GitDirty).Msg("building docker image")

	// 构建 docker 镜像
	cmd := exec.Command("docker", "build", "-t", tag, "-t", "117503445/go-template-rpc:latest", "-t", aliyunTag, "-t", "registry.cn-hangzhou.aliyuncs.com/117503445/go-template-rpc:latest", "-f", "./scripts/docker/rpc.Dockerfile", ".")
	cmd.Dir = "../.."
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Ctx(ctx).Panic().Err(err).Str("output", string(output)).Msg("failed to build docker image")
		return
	}

	log.Ctx(ctx).Info().Str("tag", tag).Msg("docker image built successfully")

	// 如果需要推送
	if cli.BuildDocker.Push {
		log.Ctx(ctx).Info().Str("tag", tag).Msg("pushing docker images")

		// 定义要推送的镜像列表
		images := []struct {
			imageName string
			registry  string
		}{
			{"117503445/go-template-rpc:latest", "Docker Hub"},
			{tag, "Docker Hub"},
			{"registry.cn-hangzhou.aliyuncs.com/117503445/go-template-rpc:latest", "Aliyun"},
			{aliyunTag, "Aliyun"},
		}

		// 并行推送所有镜像
		var wg sync.WaitGroup
		var firstErr error
		var errMu sync.Mutex

		for _, img := range images {
			wg.Add(1)
			go func(imageName, registry string) {
				defer wg.Done()
				if err := pushImage(ctx, imageName, registry); err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
				}
			}(img.imageName, img.registry)
		}

		// 等待所有推送完成
		wg.Wait()

		// 检查是否有错误
		if firstErr != nil {
			log.Ctx(ctx).Panic().Err(firstErr).Msg("failed to push docker images")
			return
		}

		log.Ctx(ctx).Info().Msg("all docker images pushed successfully")
	}

	log.Ctx(ctx).Info().Bool("push", cli.BuildDocker.Push).Msg("docker build completed")
}
