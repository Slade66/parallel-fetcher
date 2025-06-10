package task

import "github.com/google/uuid"

// DownloadTask 定义了一个完整的分布式下载任务，它将作为消息在 Redis Stream 中传递。
type DownloadTask struct {
	// 任务的唯一标识符，由 API 服务在创建任务时生成。
	ID uuid.UUID `json:"id"`

	// 要下载的文件的完整 URL。
	URL string `json:"url"`

	// 文件的保存路径，应包含完整路径和最终的文件名。
	// 例如: "/downloads/videos/my_video.mp4"
	OutputPath string `json:"output_path"`

	// 建议下载时使用的线程数。
	// Worker 服务可以将其作为参考。
	Threads int `json:"threads"`
}
