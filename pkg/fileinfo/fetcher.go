// pkg/fileinfo/fetcher.go
package fileinfo

import (
	"fmt"
	"net/http"
	"strconv"
)

// Info 包含了文件的元信息
type Info struct {
	Size          int64
	AcceptsRanges bool
}

// Get 发送 HEAD 请求以获取远程文件的信息
func Get(url string) (*Info, error) {
	resp, err := http.Head(url)
	if err != nil {
		return nil, fmt.Errorf("无法获取文件信息: %w", err)
	}
	defer resp.Body.Close()

	contentLengthStr := resp.Header.Get("Content-Length")
	if contentLengthStr == "" {
		return nil, fmt.Errorf("无法获取文件大小 (Content-Length is missing)")
	}

	size, err := strconv.ParseInt(contentLengthStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("无效的文件大小: %w", err)
	}

	return &Info{
		Size:          size,
		AcceptsRanges: resp.Header.Get("Accept-Ranges") == "bytes",
	}, nil
}
