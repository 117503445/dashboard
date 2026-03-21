# 容器构建改用Podman

## 主要内容和目的

参考 `/workspace/project/templates` 项目，将容器构建工具从 Docker 改为 Podman。

## 更改内容描述

1. **`scripts/go-scripts/build-docker.go`**
   - 将 `docker` 命令替换为 `podman`（build/push）
   - 日志消息从 "docker" 改为 "container"

2. **`scripts/tasks/build/Taskfile.yml`**
   - 更新任务描述为 "构建容器镜像 (使用 Podman)"

3. **`scripts/docker/rpc.Dockerfile`**
   - 修正二进制文件路径从 `data/rpc/rpc` 改为 `data/dashboard/dashboard`
   - 修正 ENTRYPOINT 从 `/workspace/rpc` 改为 `/workspace/dashboard`

## 验证方法和结果

```bash
go-task build:docker
```

验证结果：
- 成功构建二进制文件 `data/dashboard/dashboard`
- 成功使用 Podman 构建容器镜像 `117503445/dashboard:16b9e42-20260322-023355` (30.5 MB)
- `podman images` 显示镜像已正确创建