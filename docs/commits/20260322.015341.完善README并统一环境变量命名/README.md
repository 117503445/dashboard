# 完善 README 并统一环境变量命名

## 主要内容和目的

为项目添加完整的 README 文档，并统一环境变量命名前缀为 `DASHBOARD_`。

## 更改内容描述

1. **README.md** - 新增完整的项目说明文档
   - 功能特性介绍（Agent 管理、SSH 隧道端口转发、Code-Server 远程部署、HTTP/WebSocket 代理）
   - 系统架构图
   - 技术栈说明
   - 环境变量配置表
   - 开发和构建指南

2. **cmd/dashboard/main.go** - 统一环境变量命名
   - 将 `PORT` 改为 `DASHBOARD_PORT`，与其他环境变量命名风格保持一致

## 验证方法和结果

- 代码变更通过静态检查
- 环境变量命名风格统一，所有变量均以 `DASHBOARD_` 为前缀