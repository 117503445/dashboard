# FROM alpine:3.22.1
FROM registry.cn-hangzhou.aliyuncs.com/117503445-mirror/sync@sha256:eafc1edb577d2e9b458664a15f23ea1c370214193226069eb22921169fc7e43f
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories
RUN apk --update add ca-certificates curl tar
WORKDIR /workspace

# 下载 code-server tarball 到 data/bin
RUN mkdir -p /workspace/data/bin && \
    curl -fsSL https://github.com/coder/code-server/releases/download/v4.112.0/code-server-4.112.0-linux-amd64.tar.gz \
    -o /workspace/data/bin/code-server-4.112.0-linux-amd64.tar.gz

COPY data/dashboard/dashboard /workspace/dashboard

ENTRYPOINT [ "/workspace/dashboard" ]