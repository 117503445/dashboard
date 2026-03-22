# 添加请求ID中间件和ctx透传

## 主要内容和目的

为所有 HTTP 请求添加 request ID 中间件，实现请求级别的日志追踪能力。通过 context 透传 request ID，确保所有请求级别的日志都能关联到同一个请求。

## 更改内容描述

### 1. 新增 HTTPMiddleware 中间件 (`handler.go`)
- 为普通 HTTP 请求生成或提取 request ID（支持 X-Request-ID 和 x-fc-request-id 头）
- 注入 request ID 到 context 中（通过 `WithContext`）
- 配置 zerolog 输出 request ID
- 记录请求开始和完成的日志

### 2. 修改 server.go
- 为 API handler (`/api/agents/`) 应用 `HTTPMiddleware`
- 为代理 handler (`/proxy/agents/`) 应用 `HTTPMiddleware`
- 为静态文件 handler (`/`) 应用 `HTTPMiddleware`

### 3. 修改 handler.go
- `ProxyHandler`: 使用 `ctx := r.Context()` 获取 ctx，日志改用 `log.Ctx(ctx)`
- `proxyWebSocket`: 接受 ctx 参数，日志改用 `log.Ctx(ctx)`

### 4. 修改 code_server.go
- `SetupCodeServerHandler`: 使用 `ctx := r.Context()` 获取 ctx，透传到各个方法
- `downloadCodeServerIfNeeded`: 接受 ctx 参数，日志改用 `log.Ctx(ctx)`
- 所有请求级别日志调用改用 `log.Ctx(ctx).Info()` 等方式

## 架构说明

- **RPC handlers** (通过 connect)：继续使用 `NewCtxInterceptor` 中间件
- **普通 HTTP handlers**：使用新的 `HTTPMiddleware` 中间件
- 后台任务（如 forward.go 中的清理任务）不属于请求级别，保持原有日志方式

## 验证方法和结果

```bash
go build ./cmd/dashboard/...
```

编译成功，无错误。