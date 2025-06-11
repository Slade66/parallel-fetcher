# Dockerfile

# ---- Stage 1: Builder ----
# 使用 Go 镜像来编译你的代码
FROM golang:1.22 AS builder # 建议使用稳定版本，例如 1.22，与你项目文件中的版本保持一致
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# 编译出 api 二进制文件
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/api ./cmd/api/main.go # 明确指定 main.go 路径

# 编译出 worker 二进制文件
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/worker ./cmd/worker/main.go # 明确指定 main.go 路径


# ---- Stage 2: Final API Image ----
# 从一个干净的 alpine 镜像开始，只放入 API 程序
FROM alpine:latest AS api
WORKDIR /app
COPY --from=builder /app/api .
COPY frontend ./frontend

EXPOSE 8080
# 默认命令是启动 api
CMD ["./api"]


# ---- Stage 3: Final Worker Image ----
# 同样从一个干净的 alpine 镜像开始，只放入 Worker 程序
FROM alpine:latest AS worker
WORKDIR /app
COPY --from=builder /app/worker .
# Worker 服务不需要前端文件，所以无需复制 frontend 目录。

# 默认命令是启动 worker
CMD ["./worker"]