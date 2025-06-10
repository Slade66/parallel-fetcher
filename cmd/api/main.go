// cmd/api/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Slade66/parallel-fetcher/pkg/task"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisStreamName 是我们将任务发布到的 Redis Stream 的键名
const RedisStreamName = "download_tasks"

// RedisClient 是一个全局的 Redis 客户端实例
var RedisClient *redis.Client

// 初始化 Redis 连接
func initRedis() {
	// 从环境变量中读取 Redis 地址，如果不存在则使用默认值
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// 从环境变量读取密码，这是更安全的方式
	// 如果没有设置环境变量，则使用你提供的 "123456"
	redisPassword := os.Getenv("REDIS_PASSWORD")
	if redisPassword == "" {
		redisPassword = "123456" // 在此处设置你的密码
	}

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword, // 添加 Password 字段
		DB:       0,             // 使用默认数据库
	})

	// 使用上下文进行连接测试
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ping Redis 服务器以验证连接
	if err := RedisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ 无法连接到 Redis: %v", err)
	}

	fmt.Println("✅ 成功连接到 Redis!")
}

// downloadHandler 处理传入的下载请求
func downloadHandler(c *gin.Context) {
	// 1. 定义一个用于绑定请求 JSON 的匿名结构体
	var request struct {
		URL        string `json:"url" binding:"required"`
		OutputPath string `json:"output_path" binding:"required"`
		Threads    int    `json:"threads"`
	}

	// 2. 将请求体绑定到结构体，并进行验证
	// `binding:"required"` 确保 URL 和 OutputPath 字段必须存在
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 3. 为线程数设置默认值（如果客户端未提供或提供的值无效）
	if request.Threads <= 0 {
		request.Threads = 10 // 默认使用 10 个线程
	}

	// 4. 创建一个完整的下载任务
	task := &task.DownloadTask{
		ID:         uuid.New(),
		URL:        request.URL,
		OutputPath: request.OutputPath,
		Threads:    request.Threads,
	}

	// 5. 将任务结构体序列化为 JSON
	taskJSON, err := json.Marshal(task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法序列化任务"})
		return
	}

	// 6. 将任务发布到 Redis Stream
	err = RedisClient.XAdd(c.Request.Context(), &redis.XAddArgs{
		Stream: RedisStreamName,
		Values: map[string]interface{}{
			"payload": taskJSON,
		},
	}).Err()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法将任务发布到 Redis"})
		return
	}

	// 7. 返回成功响应
	// 使用 HTTP 202 Accepted 状态码，表示请求已被接受处理，但尚未完成
	log.Printf("📥 任务已投递到消息队列，ID: %s", task.ID)
	c.JSON(http.StatusAccepted, gin.H{
		"message": "任务已成功接收，正在排队等待处理...",
		"task_id": task.ID,
	})
}

func main() {
	// 初始化 Redis 客户端
	initRedis()

	// 创建一个默认的 Gin 路由器
	router := gin.Default()

	// 设置路由
	router.POST("/download", downloadHandler)

	fmt.Println("🚀 API 服务已启动，监听端口 :8080")
	// 启动 HTTP 服务器并监听 8080 端口
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("❌ 启动 Gin 服务失败: %v", err)
	}
}
