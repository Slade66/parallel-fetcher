// internal/downloader/downloader.go
package downloader

import (
	"fmt"
	"github.com/Slade66/parallel-fetcher/internal/client"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/Slade66/parallel-fetcher/internal/observer" // 【新增】导入 observer 包
)

// 【新增】ProgressReader 用于包装 io.Reader 来跟踪进度
type ProgressReader struct {
	io.Reader
	onProgress func(int64)
}

// Read 实现 io.Reader 接口
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	if n > 0 {
		pr.onProgress(int64(n)) // 每次读取后调用回调函数
	}
	return
}

// Downloader 结构体封装了下载任务的所有信息
type Downloader struct {
	url        string
	output     string
	threads    int
	contentLen int64
	client     *http.Client
	observers  []observer.Observer // 【新增】观察者列表
	mu         sync.Mutex          // 【新增】用于保护观察者列表的互斥锁
}

// New 创建一个新的 Downloader 实例
func New(url, output string, threads int) *Downloader {
	return &Downloader{
		url:       url,
		output:    output,
		threads:   threads,
		client:    client.GetClient(),
		observers: make([]observer.Observer, 0),
	}
}

// 【新增】AddObserver 实现了 Observable 接口，用于添加观察者
func (d *Downloader) AddObserver(o observer.Observer) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.observers = append(d.observers, o)
}

// 【新增】Notify 实现了 Observable 接口，用于通知所有观察者
func (d *Downloader) Notify(downloaded int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, obs := range d.observers {
		obs.Update(downloaded)
	}
}

// Run 启动下载流程
func (d *Downloader) Run() error {
	fmt.Println("🚀 开始下载...")

	// 1. 发送 HEAD 请求获取文件信息
	resp, err := http.Head(d.url)
	if err != nil {
		return fmt.Errorf("无法获取文件信息: %w", err)
	}
	defer resp.Body.Close()

	// 检查服务器是否支持 Range 下载
	if resp.Header.Get("Accept-Ranges") != "bytes" {
		fmt.Println("⚠️ 服务器不支持断点续传，将尝试单线程下载...")
		d.threads = 1
	}

	contentLengthStr := resp.Header.Get("Content-Length")
	if contentLengthStr == "" {
		return fmt.Errorf("无法获取文件大小 (Content-Length is missing)")
	}
	d.contentLen, err = strconv.ParseInt(contentLengthStr, 10, 64)
	if err != nil {
		return fmt.Errorf("无效的文件大小: %w", err)
	}
	fmt.Printf("文件总大小: %.2f MB\n", float64(d.contentLen)/1024/1024)

	// 创建一个临时目录来存放分片文件
	tempDir, err := os.MkdirTemp("", "fetcher-*")
	if err != nil {
		return fmt.Errorf("无法创建临时目录: %w", err)
	}
	defer os.RemoveAll(tempDir)
	// 临时文件目录信息可以不打印，让界面更干净
	fmt.Printf("临时文件目录: %s\n", tempDir)

	// 2. 计算分片并并发下载
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
			err := d.downloadPart(partPath, start, end)
			if err != nil {
				// 错误信息换行打印，避免破坏进度条
				fmt.Printf("\n❌ 下载分片 %d 失败: %v\n", partNum, err)
			}
		}(i, start, end)
	}
	wg.Wait()

	// 3. 合并所有分片文件
	fmt.Println("⏬ 所有分片下载完成，开始合并...")
	if err := d.mergeFiles(tempDir); err != nil {
		return fmt.Errorf("合并文件失败: %w", err)
	}

	fmt.Println("✅ 文件下载并合并完成！")
	return nil
}

// downloadPart 下载单个文件分片
func (d *Downloader) downloadPart(partPath string, start, end int64) error {
	req, err := http.NewRequest("GET", d.url, nil)
	if err != nil {
		return err
	}
	// 设置 Range 请求头
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回了非预期的状态码: %s", resp.Status)
	}

	file, err := os.Create(partPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 【修改】使用 ProgressReader 包装原始的响应体
	progressReader := &ProgressReader{
		Reader: resp.Body,
		onProgress: func(downloaded int64) {
			d.Notify(downloaded) // 关键：在读取到数据时，通知观察者
		},
	}

	// 【修改】从 progressReader 复制数据，而不是直接从 resp.Body
	_, err = io.Copy(file, progressReader)
	if err != nil {
		return err
	}
	return nil
}

func (d *Downloader) mergeFiles(tempDir string) error {
	outFile, err := os.Create(d.output)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for i := 0; i < d.threads; i++ {
		partPath := filepath.Join(tempDir, fmt.Sprintf("part-%d", i))
		partFile, err := os.Open(partPath)
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, partFile)
		partFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
