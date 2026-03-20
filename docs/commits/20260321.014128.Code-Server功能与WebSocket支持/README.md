# Code-Server 功能与 WebSocket 支持

## 主要内容和目的

为 Dashboard 增加 Code Server 功能，允许用户通过一键操作在远程 Agent 上自动下载、安装并启动 code-server，并通过 SSH 隧道在 iframe 中加载 VS Code 界面。同时实现 WebSocket 代理支持，确保 code-server 的实时协作功能正常工作。

## 更改内容描述

### 后端变更

1. **新增 code_server.go**
   - 实现 `SetupCodeServerHandler` API 处理器 (`POST /api/agents/{agentName}/setup-code-server`)
   - 自动下载 code-server-4.112.0 tarball 到本地 `data/bin/` 目录
   - 通过 SSH 将 tarball 传输到 Agent 的 `~/.dashboard/bin/` 目录
   - 在 Agent 上解压到 `~/.dashboard/code-server/` 目录
   - 启动 code-server 进程，监听 44444 端口
   - 创建 SSH 端口转发，返回转发后的本地端口

2. **forward.go 增强**
   - 新增 `GetSSHClient` 方法：获取或创建到指定 Agent 的 SSH 连接
   - 新增 `RunCommand` 方法：在 Agent 上执行命令并返回输出

3. **handler.go 增强**
   - 新增 `isWebSocketUpgrade` 函数：检测 WebSocket 升级请求
   - 新增 `proxyWebSocket` 函数：通过 hijack 实现 WebSocket 双向代理
   - 解决 code-server 的 WebSocket 跨域问题

4. **server.go 变更**
   - 注册 `/api/agents/` API 路由

### 前端变更

1. **AgentPanel.tsx 增强**
   - 新增 Code Server 按钮，点击后触发设置流程
   - 按钮支持加载状态（spinner 动画）
   - 新增错误提示 Banner，显示失败原因
   - 新增全屏按钮，支持 iframe 全屏显示
   - 按 ESC 键退出全屏

2. **IframeTab.tsx 变更**
   - 调整布局以容纳额外的操作按钮

3. **vite.config.ts 变更**
   - 新增 `/api` 代理配置
   - 启用 `/proxy` 路径的 WebSocket 代理

### E2E 测试

1. **新增 case4.py**
   - 测试 Code Server 启动和加载流程
   - 验证 Dashboard 在 Agent 上设置 code-server
   - 验证 VS Code 界面在 iframe 中正确加载

## 验证方法和结果

1. 启动 sshole-hub、sshole-agent 和 Dashboard 后端
2. 打开 Dashboard 页面，选择在线的 Agent
3. 点击 "Code Server" 按钮
4. 观察：
   - 按钮进入加载状态（转圈）
   - 等待约 1-2 分钟（首次需要下载）
   - 出现 `:44444` 标签页
   - VS Code 界面在 iframe 中正确加载
5. WebSocket 连接正常，无 1006 错误
6. 全屏按钮和 ESC 退出功能正常