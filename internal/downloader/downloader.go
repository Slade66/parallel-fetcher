// internal/downloader/downloader.go
package downloader

import (
	"fmt"
	"github.com/Slade66/parallel-fetcher/internal/client"
	"github.com/Slade66/parallel-fetcher/internal/observer"
	"github.com/Slade66/parallel-fetcher/internal/uploader"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Downloader 结构体封装了下载任务的所有信息
type Downloader struct {
	url           string
	output        string
	threads       int
	contentLen    int64
	acceptsRanges bool
	client        *http.Client
	observers     []observer.Observer
	mu            sync.Mutex
	uploader      *uploader.ObsUploader
}

// New 创建一个新的 Downloader 实例
func New(url, output string, threads int, size int64, acceptsRanges bool, uploader *uploader.ObsUploader) *Downloader {
	d := &Downloader{
		url:           url,
		output:        output,
		threads:       threads,
		contentLen:    size,
		acceptsRanges: acceptsRanges,
		client:        client.GetClient(),
		observers:     make([]observer.Observer, 0),
		uploader:      uploader, // 新增：赋值 uploader
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
	// 修改：将 defer os.RemoveAll(tempDir) 移动到 mergeAndUpload 内部，确保上传成功后再删除
	// defer os.RemoveAll(tempDir)

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
			partPath := filepath.Join(tempDir, fmt.Sprintf("part-%d", partNum))
			if err := d.downloadPart(partPath, start, end); err != nil {
				// 在并发的 goroutine 中打印错误，而不是返回
				fmt.Printf("\n❌ 下载分片 %d 失败: %v\n", partNum, err)
			}
		}(i, start, end)
	}
	wg.Wait()

	// 修改：调用新的合并上传方法
	fmt.Println("\n⏬ 所有分片下载完成，开始合并并上传到 OBS...")
	if err := d.mergeAndUpload(tempDir); err != nil {
		return fmt.Errorf("合并或上传文件失败: %w", err)
	}

	return nil
}
