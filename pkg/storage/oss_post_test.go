package storage

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"web-basic/internal/config"
)

// PresignPost 生成表单直传字段：浏览器 multipart POST 不触发 CORS 预检。
func TestPresignPostFields(t *testing.T) {
	o := &OSS{cfg: config.OSSStore{
		Endpoint:   "https://oss-cn-hangzhou.aliyuncs.com",
		Bucket:     "demo-bucket",
		AccessKey:  "test-ak",
		SecretKey:  "test-sk",
		SignExpire: 600,
	}}
	host, fields, err := o.PresignPost(context.Background(), "2026/01/02/resident-face/a.jpg", time.Hour, 1024)
	if err != nil {
		t.Fatalf("PresignPost error: %v", err)
	}
	if host != "https://demo-bucket.oss-cn-hangzhou.aliyuncs.com" {
		t.Fatalf("host 不正确: %s", host)
	}
	if fields["key"] != "2026/01/02/resident-face/a.jpg" {
		t.Fatalf("key 不正确: %s", fields["key"])
	}
	if fields["OSSAccessKeyId"] != "test-ak" || fields["signature"] == "" || fields["policy"] == "" {
		t.Fatalf("直传字段缺失: %+v", fields)
	}
	raw, err := base64.StdEncoding.DecodeString(fields["policy"])
	if err != nil {
		t.Fatalf("policy 不是合法 base64: %v", err)
	}
	if !strings.Contains(string(raw), "2026/01/02/resident-face/a.jpg") {
		t.Fatalf("policy 未包含对象键: %s", string(raw))
	}
}

// 未写协议时 endpoint 自动补 https，避免 https 站点被“混合内容”拦截。
func TestNewOSSDefaultsHTTPS(t *testing.T) {
	o, err := NewOSS(config.OSSStore{Endpoint: "oss-cn-hangzhou.aliyuncs.com", Bucket: "demo-bucket", AccessKey: "test-ak", SecretKey: "test-sk"})
	if err != nil {
		// 无网络环境下 SDK 初始化是纯本地操作，这里不应失败
		t.Fatalf("NewOSS error: %v", err)
	}
	if !strings.HasPrefix(o.cfg.Endpoint, "https://") {
		t.Fatalf("endpoint 应补齐 https: %s", o.cfg.Endpoint)
	}
}
