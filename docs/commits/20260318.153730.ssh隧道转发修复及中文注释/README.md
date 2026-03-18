# SSH 隧道转发修复及中文注释

## 主要内容和目的

1. **修复 SSH 隧道转发功能**：解决 web 界面访问 Agent 端口返回 502 的问题
2. **代码中文化**：将所有注释和日志信息改为中文

## 更改内容描述

### forward.go
- 将所有英文注释改为中文
- 添加更多中文注释解释代码逻辑
- 错误信息改为中文

### handler.go
- 移除 `proxyViaDirect` 兜底方法，统一使用 SSH 隧道转发
- 添加 SSH 转发未启用时的友好错误提示
- 所有注释和日志改为中文

### main.go / server.go
- 日志信息全部改为中文
- 删除冗余英文注释

### Taskfile.yml
- 添加 `DASHBOARD_SSH_PASSWORD="tunnel"` 环境变量，确保 dashboard 启动时启用 SSH 隧道转发功能

## 验证方法和结果

1. 使用 `t run:dev` 启动开发环境
2. 在 Agent 侧运行 `python -m http.server 15000`
3. 在 web 界面访问 `/proxy/agents/{agentId}/ports/15000/`
4. 验证 iframe 正常显示，HTTP 服务正常响应