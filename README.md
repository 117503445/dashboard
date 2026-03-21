# Dashboard

一个用于管理远程 Agent 的 Web 控制面板，支持 SSH 隧道端口转发和远程 Code-Server 部署。

## 功能特性

- **Agent 管理** - 查看和管理通过 sshole-hub 连接的远程 Agent 列表
- **SSH 隧道端口转发** - 通过 WebSocket 隧道建立到远程 Agent 的 SSH 连接，实现本地端口转发
- **Code-Server 远程部署** - 一键在远程 Agent 上部署和启动 code-server (VS Code Web 版)
- **HTTP/WebSocket 代理** - 代理访问远程 Agent 上的 HTTP 服务，支持 WebSocket 连接

## 架构

```
Dashboard ──WS──▶ sshole-hub ──WS──▶ sshole-agent ──TCP──▶ Agent SSH 服务
                                                              │
                                                     direct-tcpip 通道
                                                              │
                                                              ▼
                                                        Agent 目标服务端口
```

## 技术栈

- **后端**: Go + Connect RPC
- **前端**: React + Vite + TypeScript
- **通信**: gRPC (Connect) + WebSocket
- **SSH**: sshole 库建立隧道

## 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `DASHBOARD_PORT` | 服务监听端口 | `8080` |
| `DASHBOARD_SSHOLE_HUB_URL` | sshole-hub 的 HTTP 地址 | - |
| `DASHBOARD_SSHOLE_HUB_TOKEN` | sshole-hub 认证 Token | - |
| `DASHBOARD_SSH_USER` | SSH 用户名 | `root` |
| `DASHBOARD_SSH_PASSWORD` | SSH 密码认证 | - |
| `DASHBOARD_SSH_KEY_PATH` | SSH 密钥文件路径 | - |
| `DASHBOARD_MOCK_AGENTS` | 模拟 Agent 数据 (格式: `name:port:online,name2:port2:offline`) | - |

## 开发

```sh
go-task e2e:run # 运行端到端测试
```

## 许可证

[MIT](LICENSE)