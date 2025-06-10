// cmd/worker/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Slade66/parallel-fetcher/internal/downloader"
	"github.com/Slade66/parallel-fetcher/pkg/fileinfo"
	"github.com/Slade66/parallel-fetcher/pkg/task"
	"log"
	"os"
	"strings"
	"time"

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

var RedisClient *redis.Client

// 初始化 Redis 连接 (与 API 服务中的代码类似)
func initRedis() {
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
	// 为这个 Worker 实例生成一个唯一的消费者名称，通常使用主机名
	consumerName, err := os.Hostname()
	if err != nil {
		log.Printf("⚠️ 无法获取主机名，使用默认消费者名称 'worker-%d'", time.Now().Unix())
		consumerName = fmt.Sprintf("worker-%d", time.Now().Unix())
	}
	log.Printf("▶️ Worker '%s' 开始监听任务...", consumerName)

	for {
		// 1. 使用 XReadGroup 从 Stream 中阻塞式地读取一个新任务
		streams, err := RedisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    GroupName,
			Consumer: consumerName,
			Streams:  []string{StreamName, ">"}, // ">" 表示只接收从未被消费过的新消息
			Count:    1,                         // 一次只取一个任务
			Block:    0,                         // 阻塞直到有新消息
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

		log.Printf("👍 接收到新任务: [ID: %s, URL: %s]", currentTask.ID, currentTask.URL)

		// 3. 执行下载任务
		if err := executeDownload(&currentTask); err != nil {
			log.Printf("🔥 任务执行失败: [ID: %s], 错误: %v", currentTask.ID, err)
			// 注意：此处我们没有 ACK 失败的任务。
			// 这意味着消息会留在待处理列表(PEL)中，一段时间后可以被其他消费者重新获取，这是一种简单的重试机制。
			// 在生产环境中，你可能需要更复杂的错误处理，比如记录到死信队列。
		} else {
			log.Printf("✅ 任务成功完成: [ID: %s]", currentTask.ID)
			// 4. 任务成功后，发送 ACK 确认消息已被处理
			if err := RedisClient.XAck(ctx, StreamName, GroupName, message.ID).Err(); err != nil {
				log.Printf("‼️ 关键错误: 无法 ACK 任务 %s: %v", message.ID, err)
			}
		}
	}
}

// executeDownload 负责调用你现有的下载器来执行单个下载任务
func executeDownload(t *task.DownloadTask) error {
	log.Printf("🔎 正在获取文件信息: %s", t.URL)
	info, err := fileinfo.Get(t.URL) //
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 对客户端建议的线程数进行校验
	actualThreads := t.Threads
	if actualThreads <= 0 {
		actualThreads = DefaultThreads
	} else if actualThreads > MaxAllowedThreads {
		log.Printf("警告: 任务 %s 请求的线程数 (%d) 超过最大限制 (%d)，已调整。", t.ID, t.Threads, MaxAllowedThreads)
		actualThreads = MaxAllowedThreads
	}

	log.Printf("🚀 准备下载. URL: %s, 输出路径: %s, 线程数: %d", t.URL, t.OutputPath, actualThreads)

	// 创建下载器实例
	d := downloader.New(t.URL, t.OutputPath, actualThreads, info.Size, info.AcceptsRanges)

	// 重要：在 Worker 服务中，我们不再使用终端进度条观察者。
	// 所有的进度和状态都应该通过日志来记录。
	// d.AddObserver(progressBar) // 这一行被移除

	// 启动下载流程
	return d.Run() //
}

func main() {
	// 初始化
	initRedis()
	ctx := context.Background()
	ensureConsumerGroup(ctx)

	// 启动主处理循环
	processTasks(ctx)
}
