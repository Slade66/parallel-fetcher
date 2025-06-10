// main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"

	"github.com/Slade66/parallel-fetcher/internal/downloader"
	"github.com/Slade66/parallel-fetcher/internal/observer"
	"github.com/Slade66/parallel-fetcher/pkg/fileinfo"
)

func main() {
	// 1. 参数解析
	urlStr := flag.String("url", "", "要下载的文件的 URL (必须)")
	output := flag.String("output", "", "文件保存路径 (如果为空，则从URL中自动提取)")
	threads := flag.Int("threads", 10, "下载时使用的线程数")
	flag.Parse()

	// 2. 参数校验和文件名处理
	if *urlStr == "" {
		fmt.Println("错误: -url 参数是必须的")
		flag.Usage()
		os.Exit(1)
	}

	if *output == "" {
		parsedURL, err := url.Parse(*urlStr)
		if err != nil {
			log.Fatalf("❌ 无法解析提供的URL: %v", err)
		}
		filename := path.Base(parsedURL.Path)
		if filename == "" || filename == "." || filename == "/" {
			log.Fatalf("❌ 无法从URL [%s] 中自动提取有效的文件名，请使用 -output 参数手动指定。", *urlStr)
		}
		*output = filename
	}

	// 3. 获取文件信息 (使用新包)
	fmt.Println("🔎 正在获取文件信息...")
	info, err := fileinfo.Get(*urlStr)
	if err != nil {
		log.Fatalf("❌ %v", err)
	}

	// 4. 创建下载器和观察者
	d := downloader.New(*urlStr, *output, *threads, info.Size, info.AcceptsRanges)
	progressBar := observer.NewProgressBarObserver(info.Size)
	d.AddObserver(progressBar)

	// 5. 启动下载
	fmt.Println("🚀 开始下载...")
	if err := d.Run(); err != nil {
		log.Fatalf("\n❌ 下载过程中发生严重错误: %v", err)
	}
	fmt.Println("✅ 文件下载并合并完成！")
}
