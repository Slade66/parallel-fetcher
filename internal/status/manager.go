package status

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Slade66/parallel-fetcher/pkg/task"
	"github.com/redis/go-redis/v9"
	"time"
)

// StatusInfo 定义了任务状态的详细信息，用于JSON序列化
type StatusInfo struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	OutputPath string `json:"output_path"`
	Status     string `json:"status"`
	SubmitTime string `json:"submit_time"`
	FinishTime string `json:"finish_time,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Manager 结构体封装了与Redis的交互
type Manager struct {
	rdb *redis.Client
}

// NewManager 创建一个新的状态管理器实例
func NewManager(rdb *redis.Client) *Manager {
	return &Manager{rdb: rdb}
}

// taskKey 返回一个任务状态在Redis中的键名
func (m *Manager) taskKey(taskID string) string {
	return fmt.Sprintf("task:status:%s", taskID)
}

// InitTaskStatus 初始化一个新任务的状态为 "queued"
func (m *Manager) InitTaskStatus(ctx context.Context, t *task.DownloadTask) error {
	key := m.taskKey(t.ID.String())
	status := StatusInfo{
		ID:         t.ID.String(),
		URL:        t.URL,
		OutputPath: t.OutputPath, // 将 OutputPath 保存到状态中
		Status:     "queued",
		SubmitTime: time.Now().UTC().Format(time.RFC3339),
	}

	// 将 StatusInfo 结构体转换为 map[string]interface{} 以便存入 Hash
	statusMap, err := structToMap(status)
	if err != nil {
		return err
	}
	// HSet 会一次性设置多个字段
	return m.rdb.HSet(ctx, key, statusMap).Err()
}

// UpdateTaskStatus 更新任务的 'status' 字段
func (m *Manager) UpdateTaskStatus(ctx context.Context, taskID, newStatus string) error {
	key := m.taskKey(taskID)
	updateMap := map[string]interface{}{
		"status": newStatus,
	}
	// 如果任务完成或失败，则记录完成时间
	if newStatus == "completed" || newStatus == "failed" {
		updateMap["finish_time"] = time.Now().UTC().Format(time.RFC3339)
	}
	return m.rdb.HSet(ctx, key, updateMap).Err()
}

// UpdateTaskError 更新任务状态为 "failed" 并记录错误信息
func (m *Manager) UpdateTaskError(ctx context.Context, taskID, errMsg string) error {
	key := m.taskKey(taskID)
	updateMap := map[string]interface{}{
		"status":      "failed",
		"error":       errMsg,
		"finish_time": time.Now().UTC().Format(time.RFC3339),
	}
	return m.rdb.HSet(ctx, key, updateMap).Err()
}

// GetAllTasks 获取所有任务的状态信息
func (m *Manager) GetAllTasks(ctx context.Context) ([]StatusInfo, error) {
	// 1. 扫描所有符合模式的键
	keys, err := m.rdb.Keys(ctx, "task:status:*").Result()
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return []StatusInfo{}, nil
	}

	tasks := make([]StatusInfo, 0, len(keys))

	// 2. 遍历每个键，获取其 Hash 数据
	for _, key := range keys {
		// HGetAll 以 map[string]string 的形式返回哈希表的所有字段和值
		data, err := m.rdb.HGetAll(ctx, key).Result()
		if err != nil {
			// 如果某个键读取失败，记录日志并跳过它继续处理其他的
			fmt.Printf("警告: 无法读取任务状态 key '%s': %v\n", key, err)
			continue
		}

		tasks = append(tasks, StatusInfo{
			ID:         data["id"],
			URL:        data["url"],
			OutputPath: data["output_path"],
			Status:     data["status"],
			SubmitTime: data["submit_time"],
			FinishTime: data["finish_time"],
			Error:      data["error"],
		})
	}
	return tasks, nil
}

// structToMap 是一个辅助函数，用于将结构体转换为 map
func structToMap(s StatusInfo) (map[string]interface{}, error) {
	// 使用 json 标签来控制键名
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var resultMap map[string]interface{}
	err = json.Unmarshal(data, &resultMap)
	// 删除空的字段，避免在 Redis 中存储空值
	for k, v := range resultMap {
		if vs, ok := v.(string); ok && vs == "" {
			delete(resultMap, k)
		}
	}
	return resultMap, err
}
