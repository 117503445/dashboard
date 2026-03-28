package main

import "context"

type ctxKey struct{}

type AppContext struct {
	RequestID string
}

// Config 应用配置
type Config struct {
	HubURL     string
	HubToken   string
	SSHUser    string
	MockAgents string // 模拟 Agent 数据，格式: "agent1:port1:true,agent2:port2:false"
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
