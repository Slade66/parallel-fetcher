// internal/downloader/util.go
package downloader

import "io"

// ProgressReader 用于包装 io.Reader 来跟踪进度
type ProgressReader struct {
	io.Reader
	onProgress func(int64)
}

// Read 实现 io.Reader 接口
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.Reader.Read(p)
	if n > 0 {
		pr.onProgress(int64(n))
	}
	return
}
