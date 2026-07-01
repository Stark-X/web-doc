# syntax=docker/dockerfile:1.6

############################
# Stage 1: build frontend
############################
FROM node:20-alpine AS web-builder

WORKDIR /web

ARG NPM_CONFIG_REGISTRY=https://registry.npmjs.org/
ENV NPM_CONFIG_REGISTRY=${NPM_CONFIG_REGISTRY}

# 利用 layer cache：先拷依赖文件
COPY apps/web/package.json apps/web/package-lock.json* ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi

# 拷贝源码并构建
COPY apps/web/ ./

# 部署路径前缀（被反代到子路径时使用，例如 /doc/）。默认 '/'，向后兼容。
ARG VITE_BASE=/
ENV VITE_BASE=${VITE_BASE}
RUN npm run build

############################
# Stage 2: build backend
############################
FROM golang:1.22-alpine AS api-builder

WORKDIR /src/api

# 启用 module 缓存
ARG GOPROXY=https://proxy.golang.org,direct
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=mod GOPROXY=${GOPROXY}

COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

COPY apps/api/ ./
RUN go build -trimpath -ldflags "-s -w" -o /out/web-doc ./cmd/server

############################
# Stage 3: runtime
############################
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app app

WORKDIR /app

# 后端二进制
COPY --from=api-builder /out/web-doc /app/web-doc

# 前端构建产物
COPY --from=web-builder /web/dist /app/web

# 数据目录（SQLite 数据库 + 文档存储）
RUN mkdir -p /data/docs && chown -R app:app /app /data

USER app

ENV WEBDOC_ADDR=:8787 \
    WEBDOC_DB_DRIVER=sqlite \
    WEBDOC_DB_PATH=/data/webdoc.db \
    WEBDOC_STORAGE=/data/docs \
    WEBDOC_WEB_ROOT=/app/web \
    WEBDOC_ORIGIN=*

EXPOSE 8787

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8787/healthz || exit 1

ENTRYPOINT ["/app/web-doc"]
