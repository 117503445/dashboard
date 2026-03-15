package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
					Msg("request received")
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
				Msg("request done")
			return resp, err
		}
	}
}

func NewServer(config Config) *Server {
	return &Server{config: config}
}

type Server struct {
	config Config
}

func (s *Server) Healthz(ctx context.Context, req *connect.Request[rpc.HealthzRequest]) (*connect.Response[rpc.ApiResponse], error) {
	log.Ctx(ctx).Info().Msg("Healthz")
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
	// Check if using mock agents
	if s.config.MockAgents != "" {
		return s.listMockAgents(ctx)
	}

	if s.config.HubURL == "" {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("sshole-hub URL not configured"))
	}

	// Create sshole-hub client
	hubClient := rpcv1connect.NewHoleServiceClient(http.DefaultClient, s.config.HubURL)

	// Create request with auth token
	hubReq := connect.NewRequest(&rpcv1.ListAgentsRequest{})
	if s.config.HubToken != "" {
		hubReq.Header().Set("Authorization", "Bearer "+s.config.HubToken)
	}

	// Call sshole-hub ListAgents
	resp, err := hubClient.ListAgents(ctx, hubReq)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to call sshole-hub ListAgents")
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list agents: %w", err))
	}

	// Convert to our response format
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

// listMockAgents returns mock agent data for testing
func (s *Server) listMockAgents(ctx context.Context) (*connect.Response[rpc.ListAgentsResponse], error) {
	log.Ctx(ctx).Info().Msg("using mock agents")

	// Parse mock agents from config
	// Format: "agent1:port1:true,agent2:port2:false"
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

// ProxyHandler handles proxy requests to agent ports
// URL format: /proxy/agents/{agentId}/ports/{port}
func (s *Server) ProxyHandler(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /proxy/agents/{agentId}/ports/{port}/...
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/proxy/agents/"), "/")
	if len(pathParts) < 3 || pathParts[1] != "ports" {
		http.Error(w, "invalid proxy URL format, expected /proxy/agents/{agentId}/ports/{port}", http.StatusBadRequest)
		return
	}

	agentID := pathParts[0]
	portStr := pathParts[2]
	_, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "invalid port number", http.StatusBadRequest)
		return
	}

	// Get agent info from sshole-hub
	agents, err := s.getAgents(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get agents: %v", err), http.StatusInternalServerError)
		return
	}

	// Find the agent
	var hubPort int32
	for _, agent := range agents {
		if agent.AgentName == agentID {
			if !agent.Online {
				http.Error(w, "agent is offline", http.StatusServiceUnavailable)
				return
			}
			hubPort = agent.HubPort
			break
		}
	}
	if hubPort == 0 {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}

	// Build target URL
	targetURL := fmt.Sprintf("http://localhost:%d%s", hubPort, r.URL.Path)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// Create proxy request
	targetReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			targetReq.Header.Add(key, value)
		}
	}

	// Send request
	client := &http.Client{}
	resp, err := client.Do(targetReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to proxy request: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy body
	io.Copy(w, resp.Body)
}

func (s *Server) getAgents(ctx context.Context) ([]*rpc.AgentInfo, error) {
	if s.config.HubURL == "" {
		return nil, fmt.Errorf("sshole-hub URL not configured")
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

// Compile-time assertion that Server implements TemplateServiceHandler
var _ rpcconnect.TemplateServiceHandler = (*Server)(nil)
