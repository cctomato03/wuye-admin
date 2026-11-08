package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"web-basic/internal/config"
)

// Local 本地磁盘存储：落盘 baseDir，通过 urlPrefix（默认 /static）对外提供访问。
type Local struct {
	baseDir   string
	urlPrefix string
}

// NewLocal 构建本地磁盘存储。
func NewLocal(cfg config.LocalStore) *Local {
	base := cfg.BaseDir
	if base == "" {
		base = "./storage"
	}
	prefix := cfg.URLPrefix
	if prefix == "" {
		prefix = "/static"
	}
	return &Local{baseDir: base, urlPrefix: strings.TrimRight(prefix, "/")}
}

// Type 实现 Storage。
func (l *Local) Type() string { return "local" }

// Put 写入文件（自动创建日期目录）。
func (l *Local) Put(ctx context.Context, objectKey string, body io.Reader, size int64, contentType string) (string, error) {
	full, err := safePath(l.baseDir, objectKey)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(path.Dir(full), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	// 统一用 io.Copy：不依赖 size（multipart 有时给 0），也能容忍调用方未算长度
	if _, err := io.Copy(f, body); err != nil {
		return "", err
	}
	return l.publicURL(objectKey), nil
}

// Delete 删除文件。
func (l *Local) Delete(ctx context.Context, objectKey string) error {
	full, err := safePath(l.baseDir, objectKey)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// URL 本地磁盘无签名概念，直接返回静态地址。
func (l *Local) URL(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	if _, err := safePath(l.baseDir, objectKey); err != nil {
		return "", err
	}
	return l.publicURL(objectKey), nil
}

// PresignPut 本地磁盘不支持前端直传。
func (l *Local) PresignPut(ctx context.Context, objectKey string, expire time.Duration, contentType string) (string, error) {
	return "", ErrDirectUploadUnsupported
}

// Exists 判断文件是否存在。
func (l *Local) Exists(ctx context.Context, objectKey string) (bool, error) {
	full, err := safePath(l.baseDir, objectKey)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// publicURL 拼出对外访问地址（中文/空格用 path escape 保证链接可用）。
func (l *Local) publicURL(objectKey string) string {
	key := strings.TrimPrefix(strings.ReplaceAll(objectKey, "\\", "/"), "/")
	segs := strings.Split(key, "/")
	for i, s := range segs {
		segs[i] = escapeSegment(s)
	}
	return l.urlPrefix + "/" + strings.Join(segs, "/")
}

// escapeSegment 仅转义百分号、空格与常见保留字符，保留中文可读性。
func escapeSegment(s string) string {
	r := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"#", "%23",
		"?", "%3F",
	)
	return r.Replace(s)
}
