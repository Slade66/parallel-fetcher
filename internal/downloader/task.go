// internal/downloader/task.go
package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// downloadPart 下载单个文件分片
func (d *Downloader) downloadPart(partPath string, start, end int64) error {
	req, err := http.NewRequest("GET", d.url, nil)
	if err != nil {
		return err
	}
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

	progressReader := &ProgressReader{
		Reader:     resp.Body,
		onProgress: d.Notify,
	}

	_, err = io.Copy(file, progressReader)
	return err
}

// mergeFiles 合并所有分片文件
func (d *Downloader) mergeFiles(tempDir string) error {
	outFile, err := os.Create(d.output)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for i := 0; i < d.threads; i++ {
		partPath := fmt.Sprintf("%s/part-%d", tempDir, i)
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
