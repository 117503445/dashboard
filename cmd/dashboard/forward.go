package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/user"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/117503445/sshole/pkg/tunnel"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

const (
	localPortMin    = 20000
	localPortMax    = 21000
	idleTimeout     = 5 * time.Minute
	cleanupInterval = 30 * time.Second
)

type forwardKey struct {
	AgentName  string
	RemotePort int
}

// ForwardInstance 表示一个端口转发实例，维护本地监听端口到远程 Agent 端口的映射
type ForwardInstance struct {
	LocalPort  int
	RemotePort int
	AgentName  string
	HubPort    int32

	listener  net.Listener
	sshClient *ssh.Client

	lastAccess time.Time
	cancel     context.CancelFunc
	mu         sync.Mutex
}

func (f *ForwardInstance) touch() {
	f.mu.Lock()
	f.lastAccess = time.Now()
	f.mu.Unlock()
}

func (f *ForwardInstance) isIdle() bool {
	f.mu.Lock()
	idle := time.Since(f.lastAccess) > idleTimeout
	f.mu.Unlock()
	return idle
}

// ForwardManager 管理通过 WebSocket 隧道建立的 SSH 连接和本地端口转发。
//
// 连接拓扑:
//
//	Dashboard ──WS──▶ sshole-hub ──WS──▶ sshole-agent ──TCP──▶ Agent SSH 服务
//	                                                             │
//	                                                    direct-tcpip 通道
//	                                                             │
//	                                                             ▼
//	                                                       Agent 目标服务端口
type ForwardManager struct {
	mu       sync.RWMutex
	forwards map[forwardKey]*ForwardInstance
	sshConns map[string]*ssh.Client // 每个 Agent 一条 SSH 连接
	portPool map[int]bool

	sshConfig  *ssh.ClientConfig
	hubURL     string // Hub 的 HTTP 地址（如 http://localhost:9001）
	hubToken   string
	sshKeyPair *SSHKeyPair

	// sshole RPC 客户端，用于调用 AppendKnownHost
	holeClient rpcv1connect.HoleServiceClient

	ctx    context.Context
	cancel context.CancelFunc
}

func NewForwardManager(ctx context.Context, hubURL, hubToken string, sshKeyPair *SSHKeyPair) *ForwardManager {
	return NewForwardManagerWithUser(ctx, hubURL, hubToken, "", sshKeyPair)
}

func NewForwardManagerWithUser(ctx context.Context, hubURL, hubToken, sshUser string, sshKeyPair *SSHKeyPair) *ForwardManager {
	ctx, cancel := context.WithCancel(ctx)

	if sshUser == "" {
		currentUser, err := user.Current()
		if err == nil && currentUser.Username != "" {
			sshUser = currentUser.Username
		} else {
			sshUser = "root"
		}
	}

	// 创建 SSH 配置，使用公钥认证
	sshConfig := &ssh.ClientConfig{
		User:            sshUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(sshKeyPair.Signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 创建 sshole RPC 客户端
	holeClient := rpcv1connect.NewHoleServiceClient(http.DefaultClient, hubURL,
		connect.WithInterceptors(&authInterceptor{token: hubToken}))

	fm := &ForwardManager{
		forwards:   make(map[forwardKey]*ForwardInstance),
		sshConns:   make(map[string]*ssh.Client),
		portPool:   make(map[int]bool),
		sshConfig:  sshConfig,
		hubURL:     hubURL,
		hubToken:   hubToken,
		sshKeyPair: sshKeyPair,
		holeClient: holeClient,
		ctx:        ctx,
		cancel:     cancel,
	}

	for i := localPortMin; i <= localPortMax; i++ {
		fm.portPool[i] = true
	}

	go fm.cleanupLoop()

	return fm
}

// authInterceptor 为 RPC 请求添加 Authorization header
type authInterceptor struct {
	token string
}

func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if i.token != "" {
			req.Header().Set("Authorization", "Bearer "+i.token)
		}
		return next(ctx, req)
	}
}

func (i *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		return next(ctx, spec)
	}
}

func (i *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return next(ctx, conn)
	}
}

// tunnelWSURL 将 Hub 的 HTTP 地址转换为 WebSocket 隧道端点地址
func (fm *ForwardManager) tunnelWSURL() string {
	u, _ := url.Parse(fm.hubURL)
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/tunnel"
	return u.String()
}

func (fm *ForwardManager) allocatePort() (int, error) {
	for port, available := range fm.portPool {
		if available {
			fm.portPool[port] = false
			return port, nil
		}
	}
	return 0, fmt.Errorf("本地端口池耗尽（范围 %d-%d）", localPortMin, localPortMax)
}

func (fm *ForwardManager) releasePort(port int) {
	fm.portPool[port] = true
}

// getOrCreateSSHConn 通过 Hub 的 WebSocket 隧道（entry-initiated 流程）建立到 Agent 的 SSH 连接。
// 使用此流程可绕过 Hub 中 SSH-initiated 路径存在的 startForwarding 双重调用竞态问题。
func (fm *ForwardManager) getOrCreateSSHConn(agentName string, hubPort int32) (*ssh.Client, error) {
	if conn, ok := fm.sshConns[agentName]; ok {
		_, _, err := conn.SendRequest("keepalive@openssh.com", true, nil)
		if err == nil {
			return conn, nil
		}
		conn.Close()
		delete(fm.sshConns, agentName)
	}

	// 在连接前，先调用 AppendKnownHost RPC 将公钥添加到 Agent
	if err := fm.ensureAuthorizedKey(agentName); err != nil {
		log.Warn().Err(err).Str("agent", agentName).Msg("添加公钥到 Agent 失败，尝试继续连接")
	}

	tunnelURL := fm.tunnelWSURL()
	sessionID := uuid.New().String()

	header := http.Header{}
	header.Set("X-Agent", agentName)
	header.Set("X-Session", sessionID)
	if fm.hubToken != "" {
		header.Set("Authorization", "Bearer "+fm.hubToken)
	}

	dialCtx, dialCancel := context.WithTimeout(fm.ctx, 10*time.Second)
	defer dialCancel()

	ws, _, err := websocket.Dial(dialCtx, tunnelURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err != nil {
		return nil, fmt.Errorf("连接隧道 WebSocket %s 失败: %w", tunnelURL, err)
	}

	if err := tunnel.SendHandshake(dialCtx, ws, sessionID); err != nil {
		ws.Close(websocket.StatusInternalError, "握手失败")
		return nil, fmt.Errorf("隧道握手失败: %w", err)
	}

	netConn := tunnel.NetConn(fm.ctx, ws)

	addr := fmt.Sprintf("hub-tunnel:%s:%d", agentName, hubPort)
	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, fm.sshConfig)
	if err != nil {
		netConn.Close()
		return nil, fmt.Errorf("通过隧道与 %s 进行 SSH 握手失败: %w", agentName, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	fm.sshConns[agentName] = client
	log.Info().Str("agent", agentName).Msg("通过 WebSocket 隧道建立 SSH 连接成功")
	return client, nil
}

// ensureAuthorizedKey 调用 Hub 的 AppendKnownHost RPC 将公钥添加到 Agent
func (fm *ForwardManager) ensureAuthorizedKey(agentName string) error {
	ctx, cancel := context.WithTimeout(fm.ctx, 5*time.Second)
	defer cancel()

	pubKey := strings.TrimSpace(fm.sshKeyPair.PublicKeyString())
	_, err := fm.holeClient.AppendKnownHost(ctx, connect.NewRequest(&rpcv1.AppendKnownHostRequest{
		AgentName: agentName,
		PublicKey: pubKey,
	}))
	if err != nil {
		return fmt.Errorf("调用 AppendKnownHost RPC 失败: %w", err)
	}
	log.Info().Str("agent", agentName).Msg("公钥已添加到 Agent")
	return nil
}

// GetOrCreateForward 获取或创建到指定 Agent 远程端口的本地转发。
// 已有转发时直接复用；否则建立新的 SSH 隧道。
func (fm *ForwardManager) GetOrCreateForward(agentName string, remotePort int, hubPort int32) (int, error) {
	key := forwardKey{AgentName: agentName, RemotePort: remotePort}

	fm.mu.RLock()
	if fwd, ok := fm.forwards[key]; ok {
		fwd.touch()
		localPort := fwd.LocalPort
		fm.mu.RUnlock()
		return localPort, nil
	}
	fm.mu.RUnlock()

	fm.mu.Lock()
	defer fm.mu.Unlock()

	// 获取写锁后再次检查，避免重复创建
	if fwd, ok := fm.forwards[key]; ok {
		fwd.touch()
		return fwd.LocalPort, nil
	}

	sshConn, err := fm.getOrCreateSSHConn(agentName, hubPort)
	if err != nil {
		return 0, err
	}

	localPort, err := fm.allocatePort()
	if err != nil {
		return 0, err
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		fm.releasePort(localPort)
		return 0, fmt.Errorf("监听 127.0.0.1:%d 失败: %w", localPort, err)
	}

	ctx, cancel := context.WithCancel(fm.ctx)
	fwd := &ForwardInstance{
		LocalPort:  localPort,
		RemotePort: remotePort,
		AgentName:  agentName,
		HubPort:    hubPort,
		listener:   listener,
		sshClient:  sshConn,
		lastAccess: time.Now(),
		cancel:     cancel,
	}

	fm.forwards[key] = fwd
	go fm.runForward(ctx, fwd)

	log.Info().
		Str("agent", agentName).
		Int("remotePort", remotePort).
		Int("localPort", localPort).
		Msg("端口转发已创建")

	return localPort, nil
}

// runForward 在后台接受本地连接并通过 SSH 隧道转发到远程 Agent
func (fm *ForwardManager) runForward(ctx context.Context, fwd *ForwardInstance) {
	defer fwd.listener.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if tcpListener, ok := fwd.listener.(*net.TCPListener); ok {
			tcpListener.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := fwd.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-ctx.Done():
				return
			default:
				log.Error().Err(err).
					Str("agent", fwd.AgentName).
					Int("localPort", fwd.LocalPort).
					Msg("接受连接失败")
				continue
			}
		}

		go fm.handleConn(ctx, fwd, conn)
	}
}

// handleConn 处理单个转发连接：本地连接 ↔ SSH 隧道 ↔ Agent 远程端口
func (fm *ForwardManager) handleConn(_ context.Context, fwd *ForwardInstance, localConn net.Conn) {
	defer localConn.Close()
	fwd.touch()

	log.Info().
		Str("agent", fwd.AgentName).
		Int("localPort", fwd.LocalPort).
		Int("remotePort", fwd.RemotePort).
		Msg("开始处理转发连接")

	remoteAddr := fmt.Sprintf("localhost:%d", fwd.RemotePort)
	remoteConn, err := fwd.sshClient.Dial("tcp", remoteAddr)
	if err != nil {
		log.Error().Err(err).
			Str("agent", fwd.AgentName).
			Str("remoteAddr", remoteAddr).
			Msg("SSH 通道连接远程端口失败")
		return
	}
	defer remoteConn.Close()

	log.Info().
		Str("agent", fwd.AgentName).
		Int("localPort", fwd.LocalPort).
		Int("remotePort", fwd.RemotePort).
		Msg("SSH 远程连接已建立，开始双向转发")

	done := make(chan error, 2)
	go func() {
		n, err := io.Copy(remoteConn, localConn)
		log.Info().Err(err).Int64("bytes", n).
			Str("agent", fwd.AgentName).
			Msg("local -> remote io.Copy 完成")
		done <- err
	}()
	go func() {
		n, err := io.Copy(localConn, remoteConn)
		log.Info().Err(err).Int64("bytes", n).
			Str("agent", fwd.AgentName).
			Msg("remote -> local io.Copy 完成")
		done <- err
	}()

	// 等待两个方向都完成
	<-done
	<-done
	log.Info().Str("agent", fwd.AgentName).Msg("转发连接关闭")
}

func (fm *ForwardManager) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fm.ctx.Done():
			return
		case <-ticker.C:
			fm.cleanupIdle()
		}
	}
}

// cleanupIdle 清理空闲超时的转发实例和无转发引用的 SSH 连接
func (fm *ForwardManager) cleanupIdle() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	for key, fwd := range fm.forwards {
		if fwd.isIdle() {
			log.Info().
				Str("agent", fwd.AgentName).
				Int("remotePort", fwd.RemotePort).
				Int("localPort", fwd.LocalPort).
				Msg("关闭空闲转发")

			fwd.cancel()
			fm.releasePort(fwd.LocalPort)
			delete(fm.forwards, key)
		}
	}

	agentsInUse := make(map[string]bool)
	for _, fwd := range fm.forwards {
		agentsInUse[fwd.AgentName] = true
	}
	for agent, conn := range fm.sshConns {
		if !agentsInUse[agent] {
			conn.Close()
			delete(fm.sshConns, agent)
			log.Info().Str("agent", agent).Msg("关闭空闲 SSH 连接")
		}
	}
}

// GetSSHClient 获取或创建到指定 Agent 的 SSH 连接
func (fm *ForwardManager) GetSSHClient(agentName string, hubPort int32) (*ssh.Client, error) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	return fm.getOrCreateSSHConn(agentName, hubPort)
}

// RunCommand 在指定 Agent 上执行命令并返回输出
func (fm *ForwardManager) RunCommand(agentName string, hubPort int32, cmd string) (string, error) {
	client, err := fm.GetSSHClient(agentName, hubPort)
	if err != nil {
		return "", fmt.Errorf("获取 SSH 连接失败: %w", err)
	}
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建 SSH 会话失败: %w", err)
	}
	defer session.Close()
	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

// Close 关闭所有转发和 SSH 连接
func (fm *ForwardManager) Close() {
	fm.cancel()

	fm.mu.Lock()
	defer fm.mu.Unlock()

	for key, fwd := range fm.forwards {
		fwd.cancel()
		delete(fm.forwards, key)
	}
	for agent, conn := range fm.sshConns {
		conn.Close()
		delete(fm.sshConns, agent)
	}
}
