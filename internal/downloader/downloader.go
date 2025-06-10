package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Downloader 结构体封装了下载任务的所有信息
type Downloader struct {
	url        string
	output     string
	threads    int
	contentLen int64
	// 使用一个 http.Client 以便未来可以自定义配置 (例如超时)
	client *http.Client
}

// New 创建一个新的 Downloader 实例
func New(url, output string, threads int) *Downloader {
	return &Downloader{
		url:     url,
		output:  output,
		threads: threads,
		client:  &http.Client{},
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
	// 使用 defer 来确保临时目录在任务结束时被清理
	defer os.RemoveAll(tempDir)
	fmt.Printf("临时文件目录: %s\n", tempDir)

	// 2. 计算分片并并发下载
	var wg sync.WaitGroup
	blockSize := d.contentLen / int64(d.threads)

	for i := 0; i < d.threads; i++ {
		start := int64(i) * blockSize
		end := start + blockSize - 1

		// 最后一个分片需要包含所有剩余的字节
		if i == d.threads-1 {
			end = d.contentLen - 1
		}

		wg.Add(1)
		go func(partNum int, start, end int64) {
			defer wg.Done()
			partPath := filepath.Join(tempDir, fmt.Sprintf("part-%d", partNum))
			err := d.downloadPart(partPath, start, end)
			if err != nil {
				fmt.Printf("❌ 下载分片 %d 失败: %v\n", partNum, err)
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

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	// fmt.Printf("分片 %s 下载完成\n", filepath.Base(partPath)) // 可以取消注释来查看每个分片的完成情况
	return nil
}

// mergeFiles 合并临时目录中的所有分片文件到最终的输出文件
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
			// 如果某个分片文件打不开，说明下载可能出了问题
			// 这里简单返回错误，也可以实现更复杂的逻辑
			return err
		}

		_, err = io.Copy(outFile, partFile)
		partFile.Close() // 及时关闭文件句柄
		if err != nil {
			return err
		}
	}

	return nil
}
