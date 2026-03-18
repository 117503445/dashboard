package main

import "context"

type ctxKey struct{}

type AppContext struct {
	RequestID string
}

// Config holds application configuration
type Config struct {
	HubURL     string
	HubToken   string
	MockAgents string // If set, use mock agents data (format: "agent1:port1:true,agent2:port2:false")

	SSHUser     string
	SSHPassword string
	SSHKeyPath  string
}

// WithContext 注入 appContext
func WithContext(ctx context.Context, appContext AppContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, appContext)
}

// GetAppContext 获取 appContext
func GetAppContext(ctx context.Context) AppContext {
	v := ctx.Value(ctxKey{})
	if v == nil {
		return AppContext{}
	}
	return v.(AppContext)
}
