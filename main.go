// main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http" // 【新增】为了获取文件大小，需要 http 包
	"net/url"
	"os"
	"path"
	"strconv" // 【新增】

	"github.com/Slade66/parallel-fetcher/internal/downloader"
	"github.com/Slade66/parallel-fetcher/internal/observer" // 【新增】导入 observer 包
)

func main() {
	// ... (参数解析部分保持不变) ...
	urlStr := flag.String("url", "", "要下载的文件的 URL (必须)")
	output := flag.String("output", "", "文件保存路径 (如果为空，则从URL中自动提取)")
	threads := flag.Int("threads", 10, "下载时使用的线程数")
	flag.Parse()

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
		// 这行提示可以被进度条覆盖，所以可以注释掉
		// fmt.Printf("ℹ️ 未指定输出文件名，将自动使用: %s\n", *output)
	}

	// 【修改】为了创建进度条，我们需要提前获取文件总大小
	fmt.Println("🔎 正在获取文件信息...")
	resp, err := http.Head(*urlStr)
	if err != nil {
		log.Fatalf("❌ 无法获取文件信息: %v", err)
	}
	defer resp.Body.Close()

	contentLengthStr := resp.Header.Get("Content-Length")
	if contentLengthStr == "" {
		log.Fatalf("❌ 无法获取文件大小 (Content-Length is missing)")
	}
	totalSize, err := strconv.ParseInt(contentLengthStr, 10, 64)
	if err != nil {
		log.Fatalf("❌ 无效的文件大小: %v", err)
	}

	// 【修改】创建下载器和观察者，并进行注册
	d := downloader.New(*urlStr, *output, *threads)
	progressBar := observer.NewProgressBarObserver(totalSize)
	d.AddObserver(progressBar) // 将进度条观察者注册到下载器

	if err := d.Run(); err != nil {
		// 使用 log.Fatal 来打印错误并退出程序
		log.Fatalf("\n❌ 下载过程中发生严重错误: %v", err)
	}
}
