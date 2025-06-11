package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Slade66/parallel-fetcher/internal/downloader"
	"github.com/Slade66/parallel-fetcher/internal/status"
	"github.com/Slade66/parallel-fetcher/internal/uploader"
	"github.com/Slade66/parallel-fetcher/pkg/fileinfo"
	"github.com/Slade66/parallel-fetcher/pkg/task"
	"github.com/redis/go-redis/v9"
)

const (
	// 要监听的 Redis Stream 的键名
	StreamName = "download_tasks"
	// 消费者组的名称
	GroupName = "download-group"
	// Worker 内部允许的最大下载线程数，防止客户端滥用
	MaxAllowedThreads = 50
	// 默认的下载线程数
	DefaultThreads = 10
)

// 全局变量，方便在不同函数间使用
var (
	RedisClient   *redis.Client
	obsUploader   *uploader.ObsUploader
	statusManager *status.Manager
)

// initRedis 初始化 Redis 连接
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
		log.Fatalf("❌ Worker 无法连接到 Redis: %v", err)
	}
	fmt.Println("✅ Worker 成功连接到 Redis!")
}

// ensureConsumerGroup 确保消费者组存在，如果不存在则创建
func ensureConsumerGroup(ctx context.Context) {
	err := RedisClient.XGroupCreateMkStream(ctx, StreamName, GroupName, "$").Err()
	if err != nil {
		if strings.Contains(err.Error(), "BUSYGROUP") {
			log.Printf("消费者组 '%s' 已存在，无需创建。\n", GroupName)
		} else {
			log.Fatalf("❌ 无法创建消费者组: %v", err)
		}
	} else {
		log.Printf("成功创建消费者组 '%s' 并关联到 Stream '%s'。\n", GroupName, StreamName)
	}
}

// processTasks 是 Worker 的主循环，持续处理任务
func processTasks(ctx context.Context) {
	consumerName, err := os.Hostname()
	if err != nil {
		log.Printf("⚠️ 无法获取主机名，使用默认消费者名称 'worker-%d'", time.Now().Unix())
		consumerName = fmt.Sprintf("worker-%d", time.Now().Unix())
	}
	log.Printf("▶️ Worker '%s' 开始监听任务...", consumerName)

	for {
		// 1. 从 Stream 中阻塞式地读取一个新任务
		streams, err := RedisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    GroupName,
			Consumer: consumerName,
			Streams:  []string{StreamName, ">"}, // ">" 表示只接收从未被消费过的新消息
			Count:    1,
			Block:    0, // 阻塞直到有新消息
		}).Result()

		if err != nil {
			log.Printf("❌ 从 Redis Stream 读取任务失败: %v。5秒后重试...", err)
			time.Sleep(5 * time.Second)
			continue // 出错后重试
		}

		// 2. 解析收到的消息
		message := streams[0].Messages[0]
		payload := message.Values["payload"].(string)

		var currentTask task.DownloadTask
		if err := json.Unmarshal([]byte(payload), &currentTask); err != nil {
			log.Printf("‼️ 无法解析任务 payload: %v。Payload: %s", err, payload)
			// 解析失败的任务，我们直接 ACK 并跳过，防止阻塞队列
			RedisClient.XAck(ctx, StreamName, GroupName, message.ID)
			continue
		}

		log.Printf("👍 接收到新任务: [ID: %s]", currentTask.ID)

		// 3. 更新任务状态为 "processing"
		statusManager.UpdateTaskStatus(ctx, currentTask.ID.String(), "processing")

		// 4. 执行下载和上传
		if err := executeDownload(&currentTask); err != nil {
			log.Printf("🔥 任务执行失败: [ID: %s], 错误: %v", currentTask.ID, err)
			// 更新任务状态为 "failed" 并记录错误信息
			statusManager.UpdateTaskError(ctx, currentTask.ID.String(), err.Error())
			// 失败的任务我们不 ACK，以便后续可以重试或手动处理
		} else {
			log.Printf("✅ 任务成功完成: [ID: %s]", currentTask.ID)
			// 任务成功后，先更新状态为 "completed"
			statusManager.UpdateTaskStatus(ctx, currentTask.ID.String(), "completed")

			// 然后再 ACK 消息，表示任务已被完全处理
			if err := RedisClient.XAck(ctx, StreamName, GroupName, message.ID).Err(); err != nil {
				log.Printf("‼️ 关键错误: 无法 ACK 任务 %s: %v", message.ID, err)
			}
		}
	}
}

// executeDownload 负责调用下载器来执行单个下载任务
func executeDownload(t *task.DownloadTask) error {
	log.Printf("🔎 正在获取文件信息: %s", t.URL)
	info, err := fileinfo.Get(t.URL)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	actualThreads := t.Threads
	if actualThreads <= 0 {
		actualThreads = DefaultThreads
	} else if actualThreads > MaxAllowedThreads {
		log.Printf("警告: 任务 %s 请求的线程数 (%d) 超过最大限制 (%d)，已调整。", t.ID, t.Threads, MaxAllowedThreads)
		actualThreads = MaxAllowedThreads
	}

	log.Printf("🚀 准备下载. URL: %s, OBS对象键: %s, 线程数: %d", t.URL, t.OutputPath, actualThreads)

	// 创建下载器实例时，传入 obsUploader
	d := downloader.New(t.URL, t.OutputPath, actualThreads, info.Size, info.AcceptsRanges, obsUploader)

	return d.Run()
}

// main 是程序的总入口
func main() {
	// 初始化 Redis
	initRedis()
	ctx := context.Background()

	// 初始化 OBS Uploader
	obsEndpoint := os.Getenv("OBS_ENDPOINT")
	obsAk := os.Getenv("OBS_AK")
	obsSk := os.Getenv("OBS_SK")
	obsBucket := os.Getenv("OBS_BUCKET")

	if obsEndpoint == "" || obsAk == "" || obsSk == "" || obsBucket == "" {
		log.Fatalf("❌ OBS 配置不完整，请检查环境变量 OBS_ENDPOINT, OBS_AK, OBS_SK, OBS_BUCKET")
	}

	var err error
	obsUploader, err = uploader.NewObsUploader(obsEndpoint, obsAk, obsSk, obsBucket)
	if err != nil {
		log.Fatalf("❌ 初始化 OBS Uploader 失败: %v", err)
	}
	defer obsUploader.Close() // 确保程序退出时关闭客户端
	log.Println("✅ OBS Uploader 初始化成功。")

	// 初始化 Status Manager
	statusManager = status.NewManager(RedisClient)
	log.Println("✅ Status Manager 初始化成功。")

	// 确保消费者组存在
	ensureConsumerGroup(ctx)

	// 启动主处理循环，开始工作
	processTasks(ctx)
}
