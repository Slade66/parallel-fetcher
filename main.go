package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
)

func main() {
	url := flag.String("url", "", "-url 下载地址")
	output := flag.String("output", "", "-output 保存路径")
	threads := flag.Int("threads", 10, "-threads 下载时使用的线程数")
	flag.Parse()
	println(*url, *output, *threads)

	resp, err := http.Head(*url)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	contentLength, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	fmt.Println("Content-Length:", contentLength)

	acceptRanges := resp.Header.Get("Accept-Ranges")
	fmt.Println("Accept-Ranges:", acceptRanges)

	var ranges []string
	blockSize := contentLength / *threads
	for i, start := 0, 0; i < *threads; i++ {
		end := blockSize*(i+1) - 1
		if i == *threads-1 {
			end = contentLength - 1
		}
		rang := fmt.Sprintf("bytes=%d-%d", start, end)
		start = end + 1
		ranges = append(ranges, rang)
	}
	fmt.Println(blockSize, ranges)

	var wg sync.WaitGroup
	for index, rang := range ranges {
		wg.Add(1)
		go func(index int, rang string) {
			defer wg.Done()

			// 创建 GET 请求，带上 Range 头
			req, err := http.NewRequest("GET", *url, nil)
			if err != nil {
				fmt.Println("请求失败: ", err)
				return
			}
			req.Header.Set("Range", rang)

			// 发送请求
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Println("下载失败: ", err)
				return
			}
			defer resp.Body.Close()

			// 每个线程单独写文件
			filename := fmt.Sprintf("part-%d", index)
			file, err := os.Create(filename)
			if err != nil {
				fmt.Println("无法创建文件: ", err)
				return
			}
			defer file.Close()

			// 直接复制响应体到文件
			_, err = io.Copy(file, resp.Body)
			if err != nil {
				fmt.Println("写文件失败: ", err)
				return
			}

			fmt.Printf("线程 %d 下载完成！\n", index)
		}(index, rang)
	}
	wg.Wait()

	outFile, _ := os.Create(*output)
	defer outFile.Close()
	for index := range ranges {
		partName := fmt.Sprintf("part-%d", index)
		partFile, _ := os.Open(partName)
		defer partFile.Close()

		io.Copy(outFile, partFile)
	}

	fmt.Printf("合并完成！")
}
