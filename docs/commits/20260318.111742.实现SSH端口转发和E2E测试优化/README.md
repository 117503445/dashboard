# 实现 SSH 端口转发和 E2E 测试优化

## 主要内容和目的

为 Dashboard 实现 SSH 端口转发功能，使前端能够通过 iframe 访问远程 Agent 上运行的服务。同时优化 E2E 测试框架，删除冗余脚本并添加新的测试用例。

## 更改内容描述

### 1. SSH 端口转发实现

新增 `cmd/dashboard/forward.go`，实现 `ForwardManager` 组件：

- **SSH 连接管理**：维护到各 Agent 的 SSH 连接池
- **本地端口转发**：为每个 (agent, remotePort) 分配本地端口
- **生命周期管理**：支持转发的创建、查询、自动清理

### 2. 代理处理优化

修改 `cmd/dashboard/handler.go` 和 `cmd/dashboard/server.go`：

- `ProxyHandler` 改为通过 SSH 转发代理请求
- 集成 `ForwardManager`，实现按需创建转发

### 3. E2E 测试框架优化

- 删除冗余脚本 `run_test.sh` 和 `run_e2e_case3.sh`
- 在 `main.py` 中新增 `cleanup_processes()` 函数，测试前自动清理遗留进程
- 所有 E2E 测试文件注释改为中文

### 4. 新增测试用例

新增 `scripts/e2e/cases/case3.py`：SSH 端口转发端到端测试

- 启动 sshole-hub、sshole-agent、Dashboard
- 验证 Agent 发现
- 验证端口转发功能
- 截图验证转发服务内容

## 验证方法和结果

运行 E2E 测试：

```bash
task e2e:case -- case3
```

预期结果：
- Dashboard 成功通过 sshole-hub 发现 Agent
- 添加端口标签页后，iframe 成功加载转发的服务内容
- 最终截图显示 "Forwarded Service" 页面