// internal/downloader/downloader.go
package downloader

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/Slade66/parallel-fetcher/internal/client"
	"github.com/Slade66/parallel-fetcher/internal/observer"
)

// Downloader 结构体封装了下载任务的所有信息
type Downloader struct {
	url           string
	output        string
	threads       int
	contentLen    int64
	acceptsRanges bool // 新增字段
	client        *http.Client
	observers     []observer.Observer
	mu            sync.Mutex
}

// New 创建一个新的 Downloader 实例
func New(url, output string, threads int, size int64, acceptsRanges bool) *Downloader {
	d := &Downloader{
		url:           url,
		output:        output,
		threads:       threads,
		contentLen:    size,
		acceptsRanges: acceptsRanges,
		client:        client.GetClient(),
		observers:     make([]observer.Observer, 0),
	}
	// 如果服务器不支持分片下载，强制使用单线程
	if !d.acceptsRanges {
		d.threads = 1
	}
	return d
}

// AddObserver 实现了 Observable 接口，用于添加观察者
func (d *Downloader) AddObserver(o observer.Observer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.observers = append(d.observers, o)
}

// Notify 实现了 Observable 接口，用于通知所有观察者
func (d *Downloader) Notify(downloaded int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, obs := range d.observers {
		obs.Update(downloaded)
	}
}

// Run 启动下载流程
func (d *Downloader) Run() error {
	if !d.acceptsRanges {
		fmt.Println("⚠️ 服务器不支持断点续传，将使用单线程下载...")
	}
	fmt.Printf("文件总大小: %.2f MB, 使用 %d 个线程\n", float64(d.contentLen)/1024/1024, d.threads)

	tempDir, err := os.MkdirTemp("", "fetcher-*")
	if err != nil {
		return fmt.Errorf("无法创建临时目录: %w", err)
	}
	defer os.RemoveAll(tempDir)

	var wg sync.WaitGroup
	blockSize := d.contentLen / int64(d.threads)

	for i := 0; i < d.threads; i++ {
		start := int64(i) * blockSize
		end := start + blockSize - 1
		if i == d.threads-1 {
			end = d.contentLen - 1
		}

		wg.Add(1)
		go func(partNum int, start, end int64) {
			defer wg.Done()
			// 使用 filepath.Join 来安全地拼接路径，兼容不同操作系统
			partPath := filepath.Join(tempDir, fmt.Sprintf("part-%d", partNum))
			if err := d.downloadPart(partPath, start, end); err != nil {
				fmt.Printf("\n❌ 下载分片 %d 失败: %v\n", partNum, err)
			}
		}(i, start, end)
	}
	wg.Wait()

	fmt.Println("\n⏬ 所有分片下载完成，开始合并...")
	if err := d.mergeFiles(tempDir); err != nil {
		return fmt.Errorf("合并文件失败: %w", err)
	}

	return nil
}
