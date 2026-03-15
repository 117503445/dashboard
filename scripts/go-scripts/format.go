package main

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"
)

func format() {
	glog.InitZeroLog()

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	log.Ctx(ctx).Info().Msg("format")

	// Change to project root directory
	projectRoot := dirProjectRoot
	if err := os.Chdir(projectRoot); err != nil {
		log.Ctx(ctx).Error().Err(err).Str("dir", projectRoot).Msg("failed to change directory")
		os.Exit(1)
	}
	log.Ctx(ctx).Info().Str("pwd", projectRoot).Msg("changed to project root")

	// Run go mod tidy in multiple directories
	dirsToTidy := []string{
		".",
		"./scripts/go-scripts",
	}

	for _, dir := range dirsToTidy {
		log.Ctx(ctx).Info().Str("dir", dir).Msg("running go mod tidy")
		if err := runCommandInDir(ctx, dir, "go", "mod", "tidy"); err != nil {
			log.Ctx(ctx).Error().Err(err).Str("dir", dir).Msg("failed to run go mod tidy")
			os.Exit(1)
		}
	}

	// Install and run goimports-reviser
	log.Ctx(ctx).Info().Msg("checking/installing goimports-reviser")
	if err := installToolIfNeeded(ctx, "goimports-reviser", "github.com/incu6us/goimports-reviser/v3@latest"); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to install goimports-reviser")
		os.Exit(1)
	}

	log.Ctx(ctx).Info().Msg("running goimports-reviser")
	if err := runCommand(ctx, "goimports-reviser",
		"-excludes", ".git/,vendor/",
		"-company-prefixes", "aliyun/serverless",
		"-rm-unused", "-set-alias", "-format", "./..."); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to run goimports-reviser")
		os.Exit(1)
	}

	// Install and run golangci-lint
	log.Ctx(ctx).Info().Msg("checking/installing golangci-lint")
	if err := installToolIfNeeded(ctx, "golangci-lint", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.6"); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to install golangci-lint")
		os.Exit(1)
	}

	log.Ctx(ctx).Info().Msg("running golangci-lint")
	if err := runCommand(ctx, "golangci-lint", "run"); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to run golangci-lint")
		os.Exit(1)
	}

	// Check for uncommitted changes
	log.Ctx(ctx).Info().Msg("checking for uncommitted changes")
	if err := runCommand(ctx, "git", "diff-index", "--quiet", "HEAD", "--"); err != nil {
		log.Ctx(ctx).Error().Msg("### Error: Uncommitted changes detected in the working directory. ###")

		// Show the changed files
		if output, err := runCommandOutput(ctx, "git", "diff-index", "--name-status", "HEAD", "--"); err == nil {
			log.Ctx(ctx).Info().Str("changed_files", output).Msg("")
		}

		log.Ctx(ctx).Error().Msg(`
This check fails when there are any uncommitted changes, regardless of their origin.
This is to ensure that formatting and linting tools don't silently modify your work.

Possible reasons:
  1. Your code does not meet the project's formatting rules (e.g., import order).
  2. You have local changes that are not yet committed.

To resolve this:
  - Fix the formatting issues.
  - If you have intentional changes: commit them first, then re-run the format/lint script.`)
		os.Exit(1)
	}

	log.Ctx(ctx).Info().Msg("format completed successfully")
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCommandInDir(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCommandOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	return strings.TrimSpace(string(output)), err
}

func installToolIfNeeded(ctx context.Context, toolName, installPath string) error {
	// Check if tool is already available
	if err := runCommand(ctx, "which", toolName); err == nil {
		// Check version for goimports-reviser
		switch toolName {
		case "goimports-reviser":
			if err := runCommand(ctx, toolName, "-version"); err != nil {
				log.Ctx(ctx).Info().Msg("tool exists but version check failed, reinstalling")
			} else {
				log.Ctx(ctx).Info().Str("tool", toolName).Msg("tool already available")
				return nil
			}
		case "golangci-lint":
			if err := runCommand(ctx, toolName, "--version"); err != nil {
				log.Ctx(ctx).Info().Msg("tool exists but version check failed, reinstalling")
			} else {
				log.Ctx(ctx).Info().Str("tool", toolName).Msg("tool already available")
				return nil
			}
		}
	}

	// Install the tool
	log.Ctx(ctx).Info().Str("tool", toolName).Msg("installing tool")
	return runCommand(ctx, "go", "install", "-v", installPath)
}
