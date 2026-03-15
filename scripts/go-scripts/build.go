package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/117503445/goutils"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"
)

func build() {
	glog.InitZeroLog()

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	log.Ctx(ctx).Info().Msg("build")

	// Build frontend first
	buildFrontend(ctx)

	// 创建输出目录
	dirs := []string{"./data/cli", "./data/rpc", "./data/fc-event", "./data/fc-web"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Ctx(ctx).Error().Err(err).Str("dir", dir).Msg("failed to create directory")
			os.Exit(1)
		}
		log.Ctx(ctx).Info().Str("dir", dir).Msg("created directory")
	}

	// 获取构建信息
	buildInfo, err := goutils.GetBuildInfo(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to get build info")
		os.Exit(1)
	}

	// 构建程序列表
	builds := []struct {
		name string
		path string
		out  string
	}{
		{"cli", "./cmd/cli", "./data/cli/cli"},
		{"rpc", "./cmd/rpc", "./data/rpc/rpc"},
		{"fc-event", "./cmd/fc-event", "./data/fc-event/fc-event"},
		{"fc-web", "./cmd/fc-web", "./data/fc-web/fc-web"},
	}

	// 并行构建
	var wg sync.WaitGroup

	for _, build := range builds {
		wg.Add(1)
		go func(build struct {
			name string
			path string
			out  string
		}) {
			defer wg.Done()

			ctx := log.Output(glog.NewConsoleWriter(
				glog.ConsoleWriterConfig{
					RequestId: "build-" + build.name,
				})).WithContext(ctx)

			log.Ctx(ctx).Info().Msg("building")

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

			cmd := exec.Command("go", "build", "-o", build.out, "-ldflags", ldflags, build.path)
			cmd.Dir = "../.."
			cmd.Env = os.Environ()
			cmd.Env = append(cmd.Env, "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Ctx(ctx).Panic().Err(err).Str("output", string(output)).Msg("failed to build")
				return
			}

			log.Ctx(ctx).Info().Str("output", build.out).Msg("built successfully")

			// 为 fc-event 和 fc-web 创建 zip 文件
			if build.name == "fc-event" || build.name == "fc-web" {
				zipPath := "../../data/" + build.name + "/" + build.name + ".zip"
				sourcePath := "../../data/" + build.name + "/" + build.name
				if err := createFcZip(ctx, build.name, sourcePath, zipPath); err != nil {
					log.Ctx(ctx).Panic().Err(err).Str("name", build.name).Msg("failed to create zip")
					return
				}
				log.Ctx(ctx).Info().Str("zip", zipPath).Str("name", build.name).Msg("created zip")
			}
		}(build)
	}

	wg.Wait()

	log.Ctx(ctx).Info().Msg("all builds completed")
}

func createFcZip(ctx context.Context, name, sourceFile, zipPath string) error {
	// 创建 zip 文件
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// 创建 zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 打开源文件
	source, err := os.Open(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	// 在 zip 中创建文件，文件名就是程序名（根目录），并设置执行权限
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	header.SetMode(0755) // 设置执行权限 (rwxr-xr-x)
	zipEntry, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	// 复制文件内容
	_, err = io.Copy(zipEntry, source)
	if err != nil {
		return fmt.Errorf("failed to copy file to zip: %w", err)
	}

	return nil
}

func buildFrontend(ctx context.Context) {
	log.Ctx(ctx).Info().Msg("building frontend")

	// Check if fe directory exists
	if _, err := os.Stat("../../fe"); os.IsNotExist(err) {
		log.Ctx(ctx).Warn().Msg("frontend directory not found, skipping frontend build")
		return
	}

	// Run pnpm install
	cmd := exec.Command("pnpm", "install")
	cmd.Dir = "../../fe"
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Ctx(ctx).Error().Err(err).Str("output", string(output)).Msg("failed to install frontend dependencies")
		os.Exit(1)
	}
	log.Ctx(ctx).Info().Msg("frontend dependencies installed")

	// Run pnpm build
	cmd = exec.Command("pnpm", "build")
	cmd.Dir = "../../fe"
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Ctx(ctx).Error().Err(err).Str("output", string(output)).Msg("failed to build frontend")
		os.Exit(1)
	}
	log.Ctx(ctx).Info().Msg("frontend built")

	// Copy dist to cmd/rpc/dist
	srcDir := "../../fe/dist"
	dstDir := "../../cmd/rpc/dist"

	// Remove existing dist directory
	os.RemoveAll(dstDir)

	// Create destination directory
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to create rpc dist directory")
		os.Exit(1)
	}

	// Copy files
	err := copyDir(srcDir, dstDir)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to copy frontend dist to rpc")
		os.Exit(1)
	}
	log.Ctx(ctx).Info().Str("dst", dstDir).Msg("frontend dist copied to rpc")
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
