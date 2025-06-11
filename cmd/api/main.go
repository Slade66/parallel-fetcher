package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/Slade66/parallel-fetcher/internal/status"
	"github.com/Slade66/parallel-fetcher/pkg/task"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const RedisStreamName = "download_tasks"

var RedisClient *redis.Client
var statusManager *status.Manager // 新增

// initRedis 函数保持不变
func initRedis() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := RedisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ API 无法连接到 Redis: %v", err)
	}
	fmt.Println("✅ API 成功连接到 Redis!")
}

// downloadHandler 处理下载请求，并初始化任务状态
func downloadHandler(c *gin.Context) {
	var request struct {
		URL        string `json:"url" binding:"required"`
		OutputPath string `json:"output_path"`
		Threads    int    `json:"threads"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求: " + err.Error()})
		return
	}

	// 如果客户端未提供 OutputPath，则从 URL 自动生成
	if request.OutputPath == "" {
		request.OutputPath = "/app/downloads/" + path.Base(request.URL)
	}
	// 如果客户端未提供线程数，设置默认值
	if request.Threads <= 0 {
		request.Threads = 8
	}

	// 创建任务结构体
	task := &task.DownloadTask{
		ID:         uuid.New(),
		URL:        request.URL,
		OutputPath: request.OutputPath,
		Threads:    request.Threads,
	}
	taskJSON, _ := json.Marshal(task)

	// 1. 投递任务到 Stream
	err := RedisClient.XAdd(c.Request.Context(), &redis.XAddArgs{
		Stream: RedisStreamName,
		Values: map[string]interface{}{"payload": taskJSON},
	}).Err()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法将任务发布到 Redis"})
		return
	}

	// 2. 初始化任务状态记录
	if err := statusManager.InitTaskStatus(c.Request.Context(), task); err != nil {
		// 这是一个非关键性错误，只记录日志
		log.Printf("警告：无法初始化任务状态记录: %v", err)
	}

	log.Printf("📥 任务已投递到消息队列，ID: %s", task.ID)
	c.JSON(http.StatusAccepted, gin.H{
		"message": "任务已成功接收，正在排队等待处理...",
		"task_id": task.ID.String(),
	})
}

// 新增：getTasksHandler 用于处理获取所有任务列表的请求
func getTasksHandler(c *gin.Context) {
	tasks, err := statusManager.GetAllTasks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法从 Redis 获取任务列表: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func main() {
	// 初始化
	initRedis()
	statusManager = status.NewManager(RedisClient) // 初始化 statusManager

	// 设置 Gin
	router := gin.Default()

	// 新增：为 API 路由创建一个分组
	api := router.Group("/api")
	{
		api.POST("/download", downloadHandler)
		api.GET("/tasks", getTasksHandler)
	}

	// 新增：服务前端静态文件
	router.StaticFS("/", http.Dir("./frontend"))

	fmt.Println("🚀 API 服务已启动，监听端口 :8080")
	router.Run(":8080")
}
