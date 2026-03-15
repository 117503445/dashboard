# 实现 Dashboard Agent 管理功能

## 主要内容和目的

实现一个 dashboard 用于控制多台 Linux 机器，支持：
- 展示 agent 列表
- 维护多标签页 iframe，嵌入 agent 上的服务页面

## 更改内容描述

### 后端 (Go + Connect RPC)

1. **Proto 定义更新** (`pkg/rpc/template.proto`)
   - 添加 `ListAgentsRequest`、`ListAgentsResponse`、`AgentInfo` message
   - 添加 `ListAgents` RPC 方法

2. **配置支持**
   - 支持 `DASHBOARD_SSHOLE_HUB_URL` 和 `DASHBOARD_SSHOLE_HUB_TOKEN` 环境变量
   - 支持 `DASHBOARD_MOCK_AGENTS` 测试模式

3. **ListAgents RPC 实现** (`cmd/dashboard/handler.go`)
   - 调用 sshole-hub 的 `HoleService.ListAgents`
   - 支持 Bearer token 认证

4. **代理转发**
   - 实现 `/proxy/agents/{agentId}/ports/{port}` 路由

### 前端 (React + TailwindCSS)

1. **AgentList 组件** (`fe/src/components/AgentList.tsx`)
   - 显示 agent 列表
   - 显示在线状态
   - 支持选择 agent

2. **IframeTab 组件** (`fe/src/components/IframeTab.tsx`)
   - 标签页管理
   - 支持关闭和打开新窗口

3. **AgentPanel 组件** (`fe/src/components/AgentPanel.tsx`)
   - 输入框添加端口
   - iframe 展示代理页面
   - 多标签页支持

4. **主页面重构** (`fe/src/App.tsx`)
   - 左侧 agent 列表
   - 右侧 agent 面板

### 项目结构调整

- 删除 `cmd/cli`、`cmd/fc-event`、`cmd/fc-web`、`cmd/fc-web-client`
- 重命名 `cmd/rpc` 为 `cmd/dashboard`
- 更新 `scripts/go-scripts/build.go`
- 更新 `scripts/tasks/*.yml`

### E2E 测试

1. **case1** - 基本页面测试
2. **case2** - Agent 列表和端口标签测试

## 验证方法和结果

```bash
# 运行 e2e 测试
task e2e:run
```

测试结果：
```
==================================================
Test Summary:
==================================================
  case1: PASSED
  case2: PASSED
==================================================
```