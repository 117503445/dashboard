package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

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
		http.Error(w, "SSH 转发未启用（请配置 DASHBOARD_SSH_PASSWORD 或 DASHBOARD_SSH_KEY_PATH）", http.StatusServiceUnavailable)
		return
	}

	agents, err := s.getAgents(r.Context())
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
		log.Error().Err(err).Str("agent", agentID).Int("port", remotePort).Msg("创建转发失败")
		http.Error(w, fmt.Sprintf("创建转发失败: %v", err), http.StatusBadGateway)
		return
	}

	target := &url.URL{
		Scheme:   "http",
		Host:     fmt.Sprintf("127.0.0.1:%d", localPort),
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
	}
	proxy.ServeHTTP(w, r)
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
