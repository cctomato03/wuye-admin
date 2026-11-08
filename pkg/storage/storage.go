// Package storage 文件存储抽象：本地磁盘 / 阿里云 OSS 两种实现。
//
// 设计要点：
//   - 上层（service）只依赖 Storage 接口，切换存储只改配置；
//   - 对象键（objectKey）由业务层统一生成（年/月/日/[业务]/日期时间-雪花ID.扩展名），
//     存储层只负责按 key 落盘/取址，并做路径穿越兜底（safePath）；
//   - 前端直传（Presigned PUT）只有 OSS 支持，本地磁盘返回 ErrDirectUploadUnsupported。
package storage

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"web-basic/internal/config"
)

// ErrDirectUploadUnsupported 当前存储不支持前端直传。
var ErrDirectUploadUnsupported = errors.New("当前存储不支持前端直传")

// Storage 存储能力集合。
type Storage interface {
	// Type 存储类型：local / oss。
	Type() string
	// Put 写入对象，返回可访问地址。
	Put(ctx context.Context, objectKey string, body io.Reader, size int64, contentType string) (string, error)
	// Delete 删除对象（不存在视为成功）。
	Delete(ctx context.Context, objectKey string) error
	// URL 获取访问地址；ttl>0 且为私有读时返回签名地址。
	URL(ctx context.Context, objectKey string, ttl time.Duration) (string, error)
	// PresignPut 生成前端直传（PUT）签名地址。
	PresignPut(ctx context.Context, objectKey string, expire time.Duration, contentType string) (string, error)
	// Exists 判断对象是否存在。
	Exists(ctx context.Context, objectKey string) (bool, error)
}

// New 按配置构建存储实现。
func New(cfg config.Storage) (Storage, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "", "local":
		return NewLocal(cfg.Local), nil
	case "oss":
		return NewOSS(cfg.OSS)
	default:
		return nil, errors.New("不支持的存储类型: " + cfg.Type)
	}
}

// safePath 校验对象键并拼接出安全的磁盘路径：
// 拒绝空串、".."、反斜杠、绝对路径，返回 baseDir 下的完整路径。
func safePath(baseDir, objectKey string) (string, error) {
	key := strings.TrimSpace(objectKey)
	key = strings.ReplaceAll(key, "\\", "/")
	key = strings.TrimPrefix(key, "/")
	if key == "" || key == "." {
		return "", errors.New("对象键不能为空")
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", errors.New("非法对象键: " + objectKey)
		}
	}
	clean := path.Clean(key)
	if strings.HasPrefix(clean, "..") || path.IsAbs(clean) {
		return "", errors.New("非法对象键: " + objectKey)
	}
	return path.Join(strings.ReplaceAll(baseDir, "\\", "/"), clean), nil
}
