// internal/downloader/task.go
package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath" // 新增：导入 filepath
)

// downloadPart 下载单个文件分片 (此函数保持不变)
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

// mergeAndUpload 合并所有分片到临时文件，然后上传，最后清理
func (d *Downloader) mergeAndUpload(tempDir string) error {
	// 1. 创建一个临时的、用于合并的大文件
	mergedFile, err := os.CreateTemp(tempDir, "merged-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时合并文件失败: %w", err)
	}
	defer mergedFile.Close() // 确保临时文件最终被关闭

	// 2. 依次将所有分片文件写入这个临时文件
	for i := 0; i < d.threads; i++ {
		partPath := fmt.Sprintf("%s/part-%d", tempDir, i)
		partFile, err := os.Open(partPath)
		if err != nil {
			// 如果某个分片不存在，可能意味着该分片下载失败，应返回错误
			// 也需要在函数退出时清理临时目录
			os.RemoveAll(tempDir)
			return fmt.Errorf("无法打开分片文件 %s: %w", partPath, err)
		}
		_, err = io.Copy(mergedFile, partFile)
		partFile.Close() // 及时关闭文件句柄
		if err != nil {
			os.RemoveAll(tempDir)
			return fmt.Errorf("合并分片 %s 失败: %w", partPath, err)
		}
	}

	// 3. 上传这个合并好的临时文件到 OBS
	// 我们使用 d.output 作为在 OBS 中的对象键 (Object Key)
	// 使用 filepath.Base 可以去掉路径，只保留文件名
	objectKey := filepath.Base(d.output)
	if err := d.uploader.UploadFile(objectKey, mergedFile.Name()); err != nil {
		// 上传失败也需要清理临时目录
		os.RemoveAll(tempDir)
		return err
	}

	// 4. 清理所有本地临时文件
	// os.RemoveAll 会删除整个 tempDir 文件夹，包括里面的所有分片和合并后的临时文件
	return os.RemoveAll(tempDir)
}
