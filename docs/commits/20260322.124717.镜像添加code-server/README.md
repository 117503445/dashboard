# 镜像添加 code-server

## 主要内容和目的

在 Docker 镜像构建时下载 code-server 二进制文件，以便 Dashboard 可以在远程 Agent 上部署 code-server。

## 更改内容描述

修改 `scripts/docker/rpc.Dockerfile`：

1. 添加 `curl` 和 `tar` 工具
2. 下载并安装 code-server v4.112.0

```dockerfile
RUN apk --update add ca-certificates curl tar

# 下载 code-server
RUN curl -fsSL https://github.com/coder/code-server/releases/download/v4.112.0/code-server-4.112.0-linux-amd64.tar.gz | tar -xz -C /usr/local --strip-components=1
```

## 验证方法和结果

- CI 构建镜像后，下载镜像并检查 `/usr/local/bin/code-server` 是否存在