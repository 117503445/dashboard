#!/usr/bin/env sh

set -e

rm /var/run/docker.pid || true

dockerd --storage-driver=fuse-overlayfs