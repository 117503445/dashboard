package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/117503445/goutils"
	"github.com/117503445/goutils/glog"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/117503445/dashboard/internal/buildinfo"
	"github.com/117503445/dashboard/pkg/rpc"
	"github.com/117503445/dashboard/pkg/rpc/rpcconnect"
)

// HTTPMiddleware 为普通 HTTP 请求注入 request ID 到 context
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = r.Header.Get("x-fc-request-id")
			if requestID == "" {
				requestID = goutils.UUID7()
			}
		}

		ctx := WithContext(r.Context(), AppContext{
			RequestID: requestID,
		})

		ctx = log.Output(glog.NewConsoleWriter(
			glog.ConsoleWriterConfig{
				RequestId: requestID,
				DirBuild:  buildinfo.BuildDir,
			},
		)).Level(zerolog.DebugLevel).With().Caller().Logger().WithContext(ctx)

		log.Ctx(ctx).Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("收到 HTTP 请求")

		// 包装 ResponseWriter 以记录响应状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		log.Ctx(ctx).Debug().
			Int("status", wrapped.statusCode).
			Msg("HTTP 请求完成")
	})
}

// responseWriter 包装 http.ResponseWriter 以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("底层 ResponseWriter 不支持 Hijacker")
	}
	return hijacker.Hijack()
}

func (w *responseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func NewCtxInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(
			ctx context.Context,
			req connect.AnyRequest,
		) (resp connect.AnyResponse, err error) {
			requestID := ""
			if !req.Spec().IsClient {
				requestID = req.Header().Get("X-Request-ID")
				if requestID == "" {
					requestID = req.Header().Get("x-fc-request-id")
					if requestID == "" {
						requestID = goutils.UUID7()
					}
				}
				ctx = WithContext(ctx, AppContext{
					RequestID: requestID,
				})

				ctx = log.Output(glog.NewConsoleWriter(
					glog.ConsoleWriterConfig{
						RequestId: requestID,
						DirBuild:  buildinfo.BuildDir,
					},
				)).Level(zerolog.DebugLevel).With().Caller().Logger().WithContext(ctx)
				log.Ctx(ctx).Debug().
					Interface("req", req).
					Msg("收到请求")
			}
			resp, err = next(ctx, req)
			if err != nil {
				return nil, err
			}
			if resp != nil && resp.Header() != nil {
				resp.Header().Set("X-Request-ID", requestID)
			}
			log.Ctx(ctx).Debug().
				Interface("resp", resp).
				Msg("请求完成")
			return resp, err
		}
	}
}

func NewServer(config Config, forwardManager *ForwardManager) *Server {
	return &Server{config: config, forwardManager: forwardManager}
}

type Server struct {
	config         Config
	forwardManager *ForwardManager
}

func (s *Server) Healthz(ctx context.Context, req *connect.Request[rpc.HealthzRequest]) (*connect.Response[rpc.ApiResponse], error) {
	log.Ctx(ctx).Info().Msg("健康检查")
	return &connect.Response[rpc.ApiResponse]{
		Msg: &rpc.ApiResponse{
			Code:    0,
			Message: "success",
			Payload: &rpc.ApiResponse_Healthz{
				Healthz: &rpc.HealthzResponse{
					Version: buildinfo.GitVersion,
				},
			},
		},
	}, nil
}

func (s *Server) ListAgents(ctx context.Context, req *connect.Request[rpc.ListAgentsRequest]) (*connect.Response[rpc.ListAgentsResponse], error) {
	if s.config.MockAgents != "" {
		return s.listMockAgents(ctx)
	}

	if s.config.HubURL == "" {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("sshole-hub 地址未配置"))
	}

	hubClient := rpcv1connect.NewHoleServiceClient(http.DefaultClient, s.config.HubURL)

	hubReq := connect.NewRequest(&rpcv1.ListAgentsRequest{})
	if s.config.HubToken != "" {
		hubReq.Header().Set("Authorization", "Bearer "+s.config.HubToken)
	}

	resp, err := hubClient.ListAgents(ctx, hubReq)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("调用 sshole-hub ListAgents 失败")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("获取 Agent 列表失败: %w", err))
	}

	agents := make([]*rpc.AgentInfo, len(resp.Msg.Agents))
	for i, agent := range resp.Msg.Agents {
		agents[i] = &rpc.AgentInfo{
			AgentName: agent.AgentName,
			HubPort:   agent.HubPort,
			Online:    agent.Online,
		}
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].HubPort < agents[j].HubPort
	})

	return &connect.Response[rpc.ListAgentsResponse]{
		Msg: &rpc.ListAgentsResponse{
			Agents: agents,
		},
	}, nil
}

// listMockAgents 返回模拟 Agent 数据，用于测试
func (s *Server) listMockAgents(ctx context.Context) (*connect.Response[rpc.ListAgentsResponse], error) {
	log.Ctx(ctx).Info().Msg("使用模拟 Agent 数据")

	// 格式: "agent1:port1:true,agent2:port2:false"
	agents := []*rpc.AgentInfo{}

	parts := strings.Split(s.config.MockAgents, ",")
	for _, part := range parts {
		if part == "" {
			continue
		}
		fields := strings.Split(part, ":")
		if len(fields) >= 3 {
			port, err := strconv.ParseInt(fields[1], 10, 32)
			if err != nil {
				continue
			}
			online := fields[2] == "true"
			agents = append(agents, &rpc.AgentInfo{
				AgentName: fields[0],
				HubPort:   int32(port),
				Online:    online,
			})
		}
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].HubPort < agents[j].HubPort
	})

	return &connect.Response[rpc.ListAgentsResponse]{
		Msg: &rpc.ListAgentsResponse{
			Agents: agents,
		},
	}, nil
}

// ProxyHandler 处理到 Agent 端口的代理请求
// URL 格式: /proxy/agents/{agentId}/ports/{port}/...
// 通过 ForwardManager 建立 SSH 隧道转发请求到 Agent
func (s *Server) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/proxy/agents/"), "/")
	if len(pathParts) < 3 || pathParts[1] != "ports" {
		http.Error(w, "URL 格式错误，应为 /proxy/agents/{agentId}/ports/{port}", http.StatusBadRequest)
		return
	}

	agentID := pathParts[0]
	portStr := pathParts[2]
	remotePort, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "端口号无效", http.StatusBadRequest)
		return
	}

	if s.forwardManager == nil {
		http.Error(w, "SSH 转发未启用（请配置 DASHBOARD_SSHOLE_HUB_URL）", http.StatusServiceUnavailable)
		return
	}

	agents, err := s.getAgents(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("获取 Agent 列表失败: %v", err), http.StatusInternalServerError)
		return
	}

	var hubPort int32
	for _, agent := range agents {
		if agent.AgentName == agentID {
			if !agent.Online {
				http.Error(w, "Agent 离线", http.StatusServiceUnavailable)
				return
			}
			hubPort = agent.HubPort
			break
		}
	}
	if hubPort == 0 {
		http.Error(w, "Agent 未找到", http.StatusNotFound)
		return
	}

	remainingPath := "/"
	if len(pathParts) > 3 {
		remainingPath = "/" + strings.Join(pathParts[3:], "/")
	}
	rawQuery := r.URL.RawQuery

	localPort, err := s.forwardManager.GetOrCreateForward(agentID, remotePort, hubPort)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("agent", agentID).Int("port", remotePort).Msg("创建转发失败")
		http.Error(w, fmt.Sprintf("创建转发失败: %v", err), http.StatusBadGateway)
		return
	}

	targetHost := fmt.Sprintf("127.0.0.1:%d", localPort)

	if isWebSocketUpgrade(r) {
		s.proxyWebSocket(ctx, w, r, targetHost, remainingPath, rawQuery)
		return
	}

	target := &url.URL{
		Scheme:   "http",
		Host:     targetHost,
		Path:     remainingPath,
		RawQuery: rawQuery,
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = target.Path
			req.URL.RawQuery = target.RawQuery
			req.Host = target.Host
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Ctx(r.Context()).Error().Err(err).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Msg("反向代理错误")
			http.Error(w, fmt.Sprintf("代理错误：%v", err), http.StatusInternalServerError)
		},
	}
	proxy.ServeHTTP(w, r)
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// proxyWebSocket 通过 hijack 实现 WebSocket 双向代理。
// 需要显式处理 upgrade 而非依赖 httputil.ReverseProxy，
// 因为必须改写 Origin 头以通过 code-server 的跨域检查。
func (s *Server) proxyWebSocket(ctx context.Context, w http.ResponseWriter, r *http.Request, targetHost, path, rawQuery string) {
	log.Ctx(ctx).Info().Str("targetHost", targetHost).Str("path", path).Msg("开始处理 WebSocket 代理")

	backendConn, err := net.DialTimeout("tcp", targetHost, 10*time.Second)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("target", targetHost).Msg("WebSocket 后端连接失败")
		http.Error(w, fmt.Sprintf("WebSocket 后端连接失败: %v", err), http.StatusBadGateway)
		return
	}

	outReq := r.Clone(ctx)
	outReq.URL = &url.URL{Path: path, RawQuery: rawQuery}
	outReq.Host = targetHost
	outReq.RequestURI = ""
	outReq.Header.Set("Origin", "http://"+targetHost)

	log.Ctx(ctx).Debug().
		Str("method", outReq.Method).
		Str("path", outReq.URL.Path).
		Interface("headers", outReq.Header).
		Msg("发送 WebSocket 升级请求到后端")

	if err := outReq.Write(backendConn); err != nil {
		backendConn.Close()
		log.Ctx(ctx).Error().Err(err).Msg("WebSocket 请求转发失败")
		http.Error(w, fmt.Sprintf("WebSocket 请求转发失败: %v", err), http.StatusBadGateway)
		return
	}

	br := bufio.NewReader(backendConn)
	resp, err := http.ReadResponse(br, outReq)
	if err != nil {
		backendConn.Close()
		log.Ctx(ctx).Error().Err(err).Msg("WebSocket 读取后端响应失败")
		http.Error(w, fmt.Sprintf("WebSocket 后端响应失败: %v", err), http.StatusBadGateway)
		return
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		backendConn.Close()
		log.Ctx(ctx).Error().Int("status", resp.StatusCode).Str("body", string(body)).Msg("WebSocket 升级被拒绝")
		http.Error(w, fmt.Sprintf("WebSocket 升级失败: %d %s", resp.StatusCode, string(body)), resp.StatusCode)
		return
	}

	log.Ctx(ctx).Debug().Int("status", resp.StatusCode).Msg("WebSocket 升级成功，开始 hijack")

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		resp.Body.Close()
		backendConn.Close()
		http.Error(w, "不支持 WebSocket hijack", http.StatusInternalServerError)
		return
	}

	rawConn, readWriter, err := hijacker.Hijack()
	if err != nil {
		resp.Body.Close()
		backendConn.Close()
		log.Ctx(ctx).Error().Err(err).Msg("WebSocket hijack 失败")
		return
	}

	// 写入响应头到客户端
	if err := resp.Write(readWriter); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("WebSocket 写入响应头失败")
		rawConn.Close()
		backendConn.Close()
		return
	}

	// 刷新缓冲（发送响应头）
	if err := readWriter.Flush(); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("WebSocket 刷新缓冲失败")
		rawConn.Close()
		backendConn.Close()
		return
	}

	// 将 backend bufio.Reader 中已缓冲的数据（可能包含 WebSocket 帧）写入客户端
	if br.Buffered() > 0 {
		buffered := make([]byte, br.Buffered())
		n, _ := br.Read(buffered)
		log.Ctx(ctx).Debug().Int("bytes", n).Msg("从 backend bufio.Reader 读取缓冲数据")
		if n > 0 {
			if _, err := rawConn.Write(buffered[:n]); err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("WebSocket 写入缓冲数据失败")
				rawConn.Close()
				backendConn.Close()
				return
			}
		}
	}

	log.Ctx(ctx).Info().
		Str("targetHost", targetHost).
		Msg("开始 WebSocket 双向数据转发")

	// 使用原始连接进行双向数据复制
	// 当一个方向的 io.Copy 结束时，我们需要关闭另一端的写入方向
	// 这样才能正确传递 EOF 信号，让另一端也正常关闭
	done := make(chan struct{}, 2)
	var clientTCP, backendTCP *net.TCPConn
	var isClientTCP, isBackendTCP bool
	if clientTCP, isClientTCP = rawConn.(*net.TCPConn); !isClientTCP {
		clientTCP = nil
	}
	if backendTCP, isBackendTCP = backendConn.(*net.TCPConn); !isBackendTCP {
		backendTCP = nil
	}

	go func() {
		defer func() { done <- struct{}{} }()
		n, err := io.Copy(rawConn, backendConn) // backend -> client
		log.Ctx(ctx).Info().Err(err).Int64("bytes", n).Msg("WebSocket backend->client io.Copy 完成")
		// backend 发送完毕，关闭 client 的写入方向
		if clientTCP != nil {
			clientTCP.CloseWrite()
		} else {
			rawConn.Close()
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		n, err := io.Copy(backendConn, rawConn) // client -> backend
		log.Ctx(ctx).Info().Err(err).Int64("bytes", n).Msg("WebSocket client->backend io.Copy 完成")
		// client 发送完毕，关闭 backend 的写入方向
		if backendTCP != nil {
			backendTCP.CloseWrite()
		} else {
			backendConn.Close()
		}
	}()

	// 等待两个方向都完成
	<-done
	<-done
	log.Ctx(ctx).Debug().Msg("WebSocket 双向转发完成")

	// 关闭连接
	rawConn.Close()
	backendConn.Close()
	log.Ctx(ctx).Info().Str("targetHost", targetHost).Msg("WebSocket 连接关闭")
}

func (s *Server) getAgents(ctx context.Context) ([]*rpc.AgentInfo, error) {
	if s.config.HubURL == "" {
		return nil, fmt.Errorf("sshole-hub 地址未配置")
	}

	hubClient := rpcv1connect.NewHoleServiceClient(http.DefaultClient, s.config.HubURL)
	hubReq := connect.NewRequest(&rpcv1.ListAgentsRequest{})
	if s.config.HubToken != "" {
		hubReq.Header().Set("Authorization", "Bearer "+s.config.HubToken)
	}

	resp, err := hubClient.ListAgents(ctx, hubReq)
	if err != nil {
		return nil, err
	}

	agents := make([]*rpc.AgentInfo, len(resp.Msg.Agents))
	for i, agent := range resp.Msg.Agents {
		agents[i] = &rpc.AgentInfo{
			AgentName: agent.AgentName,
			HubPort:   agent.HubPort,
			Online:    agent.Online,
		}
	}
	return agents, nil
}

// 编译期断言 Server 实现了 TemplateServiceHandler 接口
var _ rpcconnect.TemplateServiceHandler = (*Server)(nil)
