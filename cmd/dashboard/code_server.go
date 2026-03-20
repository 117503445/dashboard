package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

const (
	codeServerTarball = "code-server-4.112.0-linux-amd64.tar.gz"
	codeServerDirName = "code-server-4.112.0-linux-amd64"
	codeServerURL     = "https://github.com/coder/code-server/releases/download/v4.112.0/code-server-4.112.0-linux-amd64.tar.gz"
	codeServerPort    = 44444
)

type setupCodeServerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Port    int    `json:"port,omitempty"`
}

// SetupCodeServerHandler 在 Agent 上设置并启动 code-server
// POST /api/agents/{agentName}/setup-code-server
func (s *Server) SetupCodeServerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/agents/"), "/")
	if len(pathParts) < 2 || pathParts[1] != "setup-code-server" {
		writeJSON(w, http.StatusBadRequest, setupCodeServerResponse{Message: "URL 格式错误"})
		return
	}
	agentName := pathParts[0]

	if s.forwardManager == nil {
		writeJSON(w, http.StatusServiceUnavailable, setupCodeServerResponse{Message: "SSH 转发未启用"})
		return
	}

	agents, err := s.getAgents(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
			Message: fmt.Sprintf("获取 Agent 列表失败: %v", err),
		})
		return
	}

	var hubPort int32
	for _, agent := range agents {
		if agent.AgentName == agentName {
			if !agent.Online {
				writeJSON(w, http.StatusServiceUnavailable, setupCodeServerResponse{Message: "Agent 离线"})
				return
			}
			hubPort = agent.HubPort
			break
		}
	}
	if hubPort == 0 {
		writeJSON(w, http.StatusNotFound, setupCodeServerResponse{Message: "Agent 未找到"})
		return
	}

	log.Info().Str("agent", agentName).Msg("开始设置 code-server")

	// 1. 下载 tarball 到本地（如已存在则跳过）
	localTarball := filepath.Join("data", "bin", codeServerTarball)
	if err := downloadCodeServerIfNeeded(localTarball); err != nil {
		writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
			Message: fmt.Sprintf("下载 code-server 失败: %v", err),
		})
		return
	}

	// 2. 检查 Agent 上是否已有 tarball
	remoteTarball := "~/.dashboard/bin/" + codeServerTarball
	output, err := s.forwardManager.RunCommand(agentName, hubPort,
		fmt.Sprintf("test -f %s && echo EXISTS || echo MISSING", remoteTarball))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
			Message: fmt.Sprintf("检查远程文件失败: %v", err),
		})
		return
	}

	// 3. 传输 tarball（如已存在则跳过）
	if strings.TrimSpace(output) != "EXISTS" {
		log.Info().Str("agent", agentName).Msg("传输 code-server 到 Agent")

		if _, err := s.forwardManager.RunCommand(agentName, hubPort, "mkdir -p ~/.dashboard/bin"); err != nil {
			writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
				Message: fmt.Sprintf("创建远程目录失败: %v", err),
			})
			return
		}

		sshClient, err := s.forwardManager.GetSSHClient(agentName, hubPort)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
				Message: fmt.Sprintf("获取 SSH 连接失败: %v", err),
			})
			return
		}
		if err := transferFileViaSSH(sshClient, localTarball, remoteTarball); err != nil {
			writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
				Message: fmt.Sprintf("传输文件失败: %v", err),
			})
			return
		}
		log.Info().Str("agent", agentName).Msg("code-server 传输完成")
	}

	// 4. 解压 tarball（如已存在则跳过）
	remoteDir := "~/.dashboard/code-server/" + codeServerDirName
	output, err = s.forwardManager.RunCommand(agentName, hubPort,
		fmt.Sprintf("test -d %s && echo EXISTS || echo MISSING", remoteDir))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
			Message: fmt.Sprintf("检查远程目录失败: %v", err),
		})
		return
	}

	if strings.TrimSpace(output) != "EXISTS" {
		log.Info().Str("agent", agentName).Msg("解压 code-server")
		_, err = s.forwardManager.RunCommand(agentName, hubPort,
			fmt.Sprintf("mkdir -p ~/.dashboard/code-server && tar xzf %s -C ~/.dashboard/code-server/", remoteTarball))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
				Message: fmt.Sprintf("解压失败: %v", err),
			})
			return
		}
	}

	// 5. 启动 code-server（如端口已被占用则跳过）
	output, err = s.forwardManager.RunCommand(agentName, hubPort,
		fmt.Sprintf("ss -tlnp 2>/dev/null | grep -q ':%d ' && echo IN_USE || echo FREE", codeServerPort))
	if err != nil {
		output = "FREE\n"
	}

	if strings.TrimSpace(output) != "IN_USE" {
		log.Info().Str("agent", agentName).Int("port", codeServerPort).Msg("启动 code-server")
		codeServerBin := fmt.Sprintf("%s/bin/code-server", remoteDir)
		_, _ = s.forwardManager.RunCommand(agentName, hubPort,
			fmt.Sprintf("unset VSCODE_IPC_HOOK_CLI; nohup %s --bind-addr 127.0.0.1:%d --auth none $HOME > ~/.dashboard/code-server/code-server.log 2>&1 &",
				codeServerBin, codeServerPort))

		ready := false
		for i := 0; i < 30; i++ {
			time.Sleep(1 * time.Second)
			out, err := s.forwardManager.RunCommand(agentName, hubPort,
				fmt.Sprintf("ss -tlnp 2>/dev/null | grep -q ':%d ' && echo READY || echo WAITING", codeServerPort))
			if err == nil && strings.TrimSpace(out) == "READY" {
				ready = true
				break
			}
		}
		if !ready {
			logOutput, _ := s.forwardManager.RunCommand(agentName, hubPort,
				"tail -20 ~/.dashboard/code-server/code-server.log 2>/dev/null")
			writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
				Message: fmt.Sprintf("code-server 启动超时。日志:\n%s", strings.TrimSpace(logOutput)),
			})
			return
		}
	}

	// 6. 创建端口转发
	localPort, err := s.forwardManager.GetOrCreateForward(agentName, codeServerPort, hubPort)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, setupCodeServerResponse{
			Message: fmt.Sprintf("创建端口转发失败: %v", err),
		})
		return
	}

	// 7. 验证 code-server 可通过转发访问
	verifyURL := fmt.Sprintf("http://127.0.0.1:%d/", localPort)
	client := &http.Client{Timeout: 10 * time.Second}
	for i := 0; i < 10; i++ {
		resp, err := client.Get(verifyURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				log.Info().Str("agent", agentName).Msg("code-server 验证成功")
				writeJSON(w, http.StatusOK, setupCodeServerResponse{
					Success: true,
					Message: "code-server 已启动",
					Port:    codeServerPort,
				})
				return
			}
		}
		time.Sleep(1 * time.Second)
	}

	log.Warn().Str("agent", agentName).Msg("code-server 验证超时，但转发已创建")
	writeJSON(w, http.StatusOK, setupCodeServerResponse{
		Success: true,
		Message: "code-server 转发已创建（验证超时）",
		Port:    codeServerPort,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func downloadCodeServerIfNeeded(targetPath string) error {
	if _, err := os.Stat(targetPath); err == nil {
		log.Info().Str("path", targetPath).Msg("code-server tarball 已存在，跳过下载")
		return nil
	}

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
	}

	log.Info().Str("url", codeServerURL).Str("target", targetPath).Msg("开始下载 code-server")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(codeServerURL)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp(dir, "code-server-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("写入文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名文件失败: %w", err)
	}

	log.Info().Str("target", targetPath).Msg("code-server 下载完成")
	return nil
}

func transferFileViaSSH(sshClient *ssh.Client, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer file.Close()

	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("获取 stdin 管道失败: %w", err)
	}

	go func() {
		defer stdin.Close()
		io.Copy(stdin, file)
	}()

	if err := session.Run("cat > " + remotePath); err != nil {
		return fmt.Errorf("远程写入失败: %w", err)
	}
	return nil
}
