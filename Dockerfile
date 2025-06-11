# ---- Stage 1: Builder ----
# 使用 Go 镜像来编译你的代码
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 编译出 api 二进制文件
RUN CGO_ENABLED=0 go build -o /app/api ./cmd/api

# 编译出 worker 二进制文件
RUN CGO_ENABLED=0 go build -o /app/worker ./cmd/worker


# ---- Stage 2: Final API Image ----
# 从一个干净的 alpine 镜像开始，只放入 API 程序
FROM alpine:latest AS api
WORKDIR /app
COPY --from=builder /app/api .
EXPOSE 8080
# 默认命令是启动 api
CMD ["./api"]


# ---- Stage 3: Final Worker Image ----
# 同样从一个干净的 alpine 镜像开始，只放入 Worker 程序
FROM alpine:latest AS worker
WORKDIR /app
COPY --from=builder /app/worker .
# 默认命令是启动 worker
CMD ["./worker"]