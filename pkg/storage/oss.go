package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"

	"web-basic/internal/config"
)

// OSS 阿里云对象存储实现（支持私有读 + 签名 URL + 前端直传签名）。
type OSS struct {
	cfg    config.OSSStore
	client *oss.Client
	bucket *oss.Bucket
}

// NewOSS 构建 OSS 存储：配置不全时直接报错，避免运行期才发现上传全挂。
func NewOSS(cfg config.OSSStore) (*OSS, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("OSS 配置不完整：endpoint / bucket / access_key / secret_key 均为必填")
	}
	client, err := oss.New(cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, err
	}
	return &OSS{cfg: cfg, client: client, bucket: bucket}, nil
}

// Type 实现 Storage。
func (o *OSS) Type() string { return "oss" }

// Put 上传对象。
func (o *OSS) Put(ctx context.Context, objectKey string, body io.Reader, size int64, contentType string) (string, error) {
	key := normalizeKey(objectKey)
	opts := make([]oss.Option, 0, 2)
	if contentType != "" {
		opts = append(opts, oss.ContentType(contentType))
	}
	if err := o.bucket.PutObject(key, body, opts...); err != nil {
		return "", err
	}
	return o.URL(ctx, key, 0)
}

// Delete 删除对象（不存在视为成功）。
func (o *OSS) Delete(ctx context.Context, objectKey string) error {
	key := normalizeKey(objectKey)
	err := o.bucket.DeleteObject(key)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "nosuchkey") {
		return nil
	}
	return err
}

// URL 返回访问地址：公有读直接用域名，私有读返回默认有效期的签名地址。
func (o *OSS) URL(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	key := normalizeKey(objectKey)
	if o.cfg.Private {
		if ttl <= 0 {
			ttl = time.Duration(defaultInt(o.cfg.SignExpire, 900)) * time.Second
		}
		return o.bucket.SignURL(key, oss.HTTPGet, int64(ttl.Seconds()))
	}
	if o.cfg.Domain != "" {
		return strings.TrimRight(o.cfg.Domain, "/") + "/" + key, nil
	}
	// 未配置自定义域名时按 endpoint 规则拼接（bucket.endpoint）
	host := strings.TrimPrefix(strings.TrimPrefix(o.cfg.Endpoint, "https://"), "http://")
	scheme := "https://"
	if strings.HasPrefix(o.cfg.Endpoint, "http://") {
		scheme = "http://"
	}
	return scheme + o.cfg.Bucket + "." + host + "/" + key, nil
}

// PresignPut 生成前端直传（PUT）签名。
func (o *OSS) PresignPut(ctx context.Context, objectKey string, expire time.Duration, contentType string) (string, error) {
	key := normalizeKey(objectKey)
	if expire <= 0 {
		expire = time.Duration(defaultInt(o.cfg.SignExpire, 900)) * time.Second
	}
	opts := make([]oss.Option, 0, 1)
	if contentType != "" {
		opts = append(opts, oss.ContentType(contentType))
	}
	return o.bucket.SignURL(key, oss.HTTPPut, int64(expire.Seconds()), opts...)
}

// Exists 判断对象是否存在（直传回执校验用）。
func (o *OSS) Exists(ctx context.Context, objectKey string) (bool, error) {
	return o.bucket.IsObjectExist(normalizeKey(objectKey))
}

// normalizeKey 去掉前导斜杠，保证本地/OSS 两种实现的对象键口径一致。
func normalizeKey(key string) string {
	return strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(key), "\\", "/"), "/")
}

func defaultInt(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
