// internal/client/singleton_client.go
package client

import (
	"net/http"
	"sync"
	"time"
)

var (
	instance *http.Client
	once     sync.Once
)

// GetClient 返回 http.Client 的单例
// 在第一次被调用时，它会初始化一个自定义配置的 http.Client
// 后续所有调用都将返回这同一个实例
func GetClient() *http.Client {
	once.Do(func() {
		// 在这里可以对 http.Client 进行精细化配置
		// 例如设置超时时间、自定义 Transport 等
		instance = &http.Client{
			Timeout: 30 * time.Second, // 例如，为所有请求设置一个30秒的超时
		}
	})
	return instance
}
