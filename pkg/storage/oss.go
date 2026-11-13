package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
	// 没写协议时默认 https：阿里云 SDK 对无协议 endpoint 会生成 http 预签名 URL，
	// 在 https 站点里会被浏览器按"混合内容"拦截（表现为 PUT 没有请求体、随后回退服务端上传）。
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	cfg.Endpoint = endpoint
	client, err := oss.New(endpoint, cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("OSS 初始化失败(endpoint=%s): %w", endpoint, err)
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
		return "", fmt.Errorf("OSS PutObject 失败(bucket=%s, key=%s): %w", o.cfg.Bucket, key, err)
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
	u, err := o.bucket.SignURL(key, oss.HTTPPut, int64(expire.Seconds()), opts...)
	if err != nil {
		return "", fmt.Errorf("OSS 直传签名失败(bucket=%s, key=%s): %w", o.cfg.Bucket, key, err)
	}
	return u, nil
}

// Exists 判断对象是否存在（直传回执校验用）。
// PresignPost 生成表单直传（PostObject）所需的 host 与字段。
// 浏览器用 multipart/form-data POST 上传属于"简单请求"，不触发 CORS 预检；
// 响应是否可读不影响上传结果，前端随后调 /files/oss-record，由服务端 HEAD 校验对象存在后落库。
func (o *OSS) PresignPost(ctx context.Context, objectKey string, expire time.Duration, maxSize int64) (string, map[string]string, error) {
	key := normalizeKey(objectKey)
	if expire <= 0 {
		expire = time.Duration(defaultInt(o.cfg.SignExpire, 900)) * time.Second
	}
	if maxSize <= 0 {
		maxSize = 20 * 1024 * 1024
	}
	expiration := time.Now().Add(expire).UTC().Format("2006-01-02T15:04:05.000Z")
	doc := map[string]any{
		"expiration": expiration,
		"conditions": []any{
			map[string]string{"bucket": o.cfg.Bucket},
			map[string]string{"key": key},
			[]any{"content-length-range", 1, maxSize},
			map[string]string{"success_action_status": "200"},
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", nil, err
	}
	policy := base64.StdEncoding.EncodeToString(raw)
	mac := hmac.New(sha1.New, []byte(o.cfg.SecretKey))
	mac.Write([]byte(policy))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	host := strings.TrimRight(o.cfg.Endpoint, "/")
	scheme := "https://"
	if strings.HasPrefix(host, "http://") {
		scheme = "http://"
	}
	plainHost := strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	if o.cfg.Domain != "" {
		host = strings.TrimRight(o.cfg.Domain, "/")
	} else {
		host = scheme + o.cfg.Bucket + "." + plainHost
	}
	fields := map[string]string{
		"key":                   key,
		"policy":                policy,
		"OSSAccessKeyId":        o.cfg.AccessKey,
		"signature":             signature,
		"success_action_status": "200",
	}
	return host, fields, nil
}

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
