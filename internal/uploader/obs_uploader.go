// internal/uploader/obs_uploader.go
package uploader

import (
	"fmt"
	"github.com/huaweicloud/huaweicloud-sdk-go-obs/obs"
)

// ObsUploader 结构体封装了 OBS 客户端和配置
type ObsUploader struct {
	client *obs.ObsClient
	bucket string
}

// NewObsUploader 根据官方文档创建一个新的 OBS 上传器实例
func NewObsUploader(endpoint, ak, sk, bucket string) (*ObsUploader, error) {
	// obs.New 是创建客户端实例的函数
	client, err := obs.New(ak, sk, endpoint)
	if err != nil {
		return nil, fmt.Errorf("无法创建 OBS 客户端: %w", err)
	}

	return &ObsUploader{
		client: client,
		bucket: bucket,
	}, nil
}

// UploadFile 将指定路径的本地文件上传到 OBS
func (u *ObsUploader) UploadFile(objectKey, filePath string) error {
	// PutFileInput 是上传本地文件所需的参数结构体
	input := &obs.PutFileInput{}
	input.Bucket = u.bucket
	input.Key = objectKey       // objectKey 是文件在 OBS 桶中的名字/路径
	input.SourceFile = filePath // 本地文件的路径

	// 调用 PutFile 方法执行上传
	output, err := u.client.PutFile(input)
	if err != nil {
		// 尝试解析 OBS 返回的详细错误信息
		if obsError, ok := err.(obs.ObsError); ok {
			return fmt.Errorf("上传失败，OBS错误码: %s, 错误信息: %s", obsError.Code, obsError.Message)
		}
		return fmt.Errorf("上传文件到 OBS 失败: %w", err)
	}

	fmt.Printf("文件 '%s' 已成功上传到 OBS 桶 '%s'，对象键为 '%s' (ETag: %s)\n", filePath, u.bucket, objectKey, output.ETag)
	return nil
}

// Close 关闭客户端连接
func (u *ObsUploader) Close() {
	if u.client != nil {
		u.client.Close()
	}
}
