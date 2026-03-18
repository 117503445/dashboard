package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

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

// ForwardManager manages SSH connections over WebSocket tunnels and local port forwarding.
//
// Connection topology:
//
//	Dashboard ──WS──▶ sshole-hub ──WS──▶ sshole-agent ──TCP──▶ Agent SSH server
//	                                                             │
//	                                                    direct-tcpip channel
//	                                                             │
//	                                                             ▼
//	                                                       Agent service port
type ForwardManager struct {
	mu       sync.RWMutex
	forwards map[forwardKey]*ForwardInstance
	sshConns map[string]*ssh.Client
	portPool map[int]bool

	sshConfig *ssh.ClientConfig
	hubURL    string // HTTP URL of the hub (e.g. http://localhost:9002)
	hubToken  string

	ctx    context.Context
	cancel context.CancelFunc
}

func NewForwardManager(ctx context.Context, hubURL, hubToken string, sshConfig *ssh.ClientConfig) *ForwardManager {
	ctx, cancel := context.WithCancel(ctx)
	fm := &ForwardManager{
		forwards:  make(map[forwardKey]*ForwardInstance),
		sshConns:  make(map[string]*ssh.Client),
		portPool:  make(map[int]bool),
		sshConfig: sshConfig,
		hubURL:    hubURL,
		hubToken:  hubToken,
		ctx:       ctx,
		cancel:    cancel,
	}

	for i := localPortMin; i <= localPortMax; i++ {
		fm.portPool[i] = true
	}

	go fm.cleanupLoop()

	return fm
}

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
	return 0, fmt.Errorf("no available local ports in range %d-%d", localPortMin, localPortMax)
}

func (fm *ForwardManager) releasePort(port int) {
	fm.portPool[port] = true
}

// getOrCreateSSHConn establishes an SSH connection to the agent via the hub's
// WebSocket tunnel (entry-initiated flow). This avoids the hub's SSH-initiated
// code path which has a double-startForwarding race condition.
func (fm *ForwardManager) getOrCreateSSHConn(agentName string, hubPort int32) (*ssh.Client, error) {
	if conn, ok := fm.sshConns[agentName]; ok {
		_, _, err := conn.SendRequest("keepalive@openssh.com", true, nil)
		if err == nil {
			return conn, nil
		}
		conn.Close()
		delete(fm.sshConns, agentName)
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
		return nil, fmt.Errorf("dial tunnel WebSocket %s: %w", tunnelURL, err)
	}

	if err := tunnel.SendHandshake(dialCtx, ws, sessionID); err != nil {
		ws.Close(websocket.StatusInternalError, "handshake failed")
		return nil, fmt.Errorf("tunnel handshake: %w", err)
	}

	netConn := tunnel.NetConn(fm.ctx, ws)

	addr := fmt.Sprintf("hub-tunnel:%s:%d", agentName, hubPort)
	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, fm.sshConfig)
	if err != nil {
		netConn.Close()
		return nil, fmt.Errorf("SSH handshake over tunnel to %s: %w", agentName, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	fm.sshConns[agentName] = client
	log.Info().Str("agent", agentName).Msg("SSH connection established via WebSocket tunnel")
	return client, nil
}

// GetOrCreateForward returns the local port for forwarding to the given agent's remote port.
// If a forward already exists, it is reused; otherwise a new SSH tunnel is established.
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
		return 0, fmt.Errorf("failed to listen on 127.0.0.1:%d: %w", localPort, err)
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
		Msg("created port forward")

	return localPort, nil
}

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
					Msg("accept failed")
				continue
			}
		}

		go fm.handleConn(ctx, fwd, conn)
	}
}

func (fm *ForwardManager) handleConn(_ context.Context, fwd *ForwardInstance, localConn net.Conn) {
	defer localConn.Close()
	fwd.touch()

	remoteAddr := fmt.Sprintf("localhost:%d", fwd.RemotePort)
	remoteConn, err := fwd.sshClient.Dial("tcp", remoteAddr)
	if err != nil {
		log.Error().Err(err).
			Str("agent", fwd.AgentName).
			Str("remoteAddr", remoteAddr).
			Msg("SSH dial to remote port failed")
		return
	}
	defer remoteConn.Close()

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(remoteConn, localConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(localConn, remoteConn)
		done <- struct{}{}
	}()

	<-done
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

func (fm *ForwardManager) cleanupIdle() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	for key, fwd := range fm.forwards {
		if fwd.isIdle() {
			log.Info().
				Str("agent", fwd.AgentName).
				Int("remotePort", fwd.RemotePort).
				Int("localPort", fwd.LocalPort).
				Msg("closing idle forward")

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
			log.Info().Str("agent", agent).Msg("closed idle SSH connection")
		}
	}
}

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
