package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"

	"github.com/Slade66/parallel-fetcher/internal/downloader"
)

func main() {
	// 1. 解析命令行参数
	// 为了避免和标准库的 url 包名冲突，将变量名从 url 改为 urlStr
	urlStr := flag.String("url", "", "要下载的文件的 URL (必须)")
	// 更新了 output 参数的描述信息
	output := flag.String("output", "", "文件保存路径 (如果为空，则从URL中自动提取)")
	threads := flag.Int("threads", 10, "下载时使用的线程数")
	flag.Parse()

	// 2. 校验和处理参数
	if *urlStr == "" {
		fmt.Println("错误: -url 参数是必须的")
		flag.Usage()
		os.Exit(1)
	}

	// 如果 -output 参数为空，则从 URL 中自动提取文件名
	if *output == "" {
		parsedURL, err := url.Parse(*urlStr)
		if err != nil {
			// 如果 URL 本身就有问题，直接报错退出
			log.Fatalf("❌ 无法解析提供的URL: %v", err)
		}
		// 使用 path.Base 获取 URL 路径的最后一部分
		filename := path.Base(parsedURL.Path)
		// 做一个简单的检查，防止 URL 以 "/" 结尾导致文件名为空或 "."
		if filename == "" || filename == "." || filename == "/" {
			log.Fatalf("❌ 无法从URL [%s] 中自动提取有效的文件名，请使用 -output 参数手动指定。", *urlStr)
		}
		*output = filename
		fmt.Printf("ℹ️ 未指定输出文件名，将自动使用: %s\n", *output)
	}

	// 3. 创建并运行下载器
	d := downloader.New(*urlStr, *output, *threads)
	if err := d.Run(); err != nil {
		// 使用 log.Fatal 来打印错误并退出程序
		log.Fatalf("❌ 下载过程中发生严重错误: %v", err)
	}
}
