// Package imagehosting 提供统一的图床上传接口。
//
// 支持 7 种图床后端，按配置顺序依次尝试，第一个成功的返回结果：
//  1. COS (腾讯云对象存储)
//  2. Bilibili (B站图床)
//  3. QQ频道 (通过发消息获取 qpic.cn 链接)
//  4. ChatGLM (智谱免费图床)
//  5. Ukaka (免费签名上传)
//  6. 星野 (免费签名上传)
//  7. Nature (腾讯COS直传, 密钥内置)
//
// 使用方式:
//
//	url, err := imagehosting.Upload(base64Data, "image.png")
//	if err != nil {
//	    // fallback
//	}
package imagehosting

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/mylog"
)

// ---------- 统一上传接口 ----------

// Upload 解码 base64 图片后按配置优先级依次尝试各图床。
// 返回公开可访问的图片 URL。
func Upload(base64Data string, filename string) (string, error) {
	imageBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}

	// 按优先级依次尝试
	uploaders := []struct {
		name string
		fn   func([]byte, string) (string, error)
	}{
		{"COS", tryCOS},
		{"Bilibili", tryBilibili},
		{"QQChannel", tryQQChannel},
		{"ChatGLM", tryChatGLM},
		{"Ukaka", tryUkaka},
		{"Xingye", tryXingye},
		{"Nature", tryNature},
	}

	for _, u := range uploaders {
		if !isEnabled(u.name) {
			continue
		}
		url, err := u.fn(imageBytes, filename)
		if err == nil && url != "" {
			mylog.Printf("图床 [%s] 上传成功", u.name)
			return url, nil
		}
		if err != nil {
			mylog.Printf("图床 [%s] 上传失败: %v", u.name, err)
		}
	}

	return "", fmt.Errorf("所有图床均上传失败")
}

// UploadBytes 直接上传 bytes（无需 base64 编解码）
func UploadBytes(imageData []byte, filename string) (string, error) {
	uploaders := []struct {
		name string
		fn   func([]byte, string) (string, error)
	}{
		{"COS", tryCOS},
		{"Bilibili", tryBilibili},
		{"QQChannel", tryQQChannel},
		{"ChatGLM", tryChatGLM},
		{"Ukaka", tryUkaka},
		{"Xingye", tryXingye},
		{"Nature", tryNature},
	}

	for _, u := range uploaders {
		if !isEnabled(u.name) {
			continue
		}
		url, err := u.fn(imageData, filename)
		if err == nil && url != "" {
			mylog.Printf("图床 [%s] 上传成功", u.name)
			return url, nil
		}
		if err != nil {
			mylog.Printf("图床 [%s] 上传失败: %v", u.name, err)
		}
	}

	return "", fmt.Errorf("所有图床均上传失败")
}

// ---------- 配置检查 ----------

func isEnabled(name string) bool {
	switch name {
	case "COS":
		return config.GetImageHostingCOS().Enabled
	case "Bilibili":
		return config.GetImageHostingBilibili().Enabled
	case "QQChannel":
		return config.GetImageHostingQQChannel().Enabled
	case "ChatGLM":
		return config.GetImageHostingChatGLM().Enabled
	case "Ukaka":
		return config.GetImageHostingUkaka().Enabled
	case "Xingye":
		return config.GetImageHostingXingye().Enabled
	case "Nature":
		return config.GetImageHostingNature().Enabled
	}
	return false
}

// ---------- 辅助函数 ----------

// detectMIME 从图片 bytes 检测 MIME 类型
func detectMIME(data []byte) string {
	if len(data) < 12 {
		return "image/jpeg"
	}
	switch {
	case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}):
		return "image/png"
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")):
		return "image/gif"
	case len(data) > 12 && string(data[8:12]) == "WEBP":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

// detectExt 从图片 bytes 检测扩展名
func detectExt(data []byte) string {
	switch detectMIME(data) {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	default:
		return "jpg"
	}
}

// getImageDimensions 从 bytes 读取图片尺寸
func getImageDimensions(data []byte) (int, int) {
	img, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return img.Width, img.Height
}

// httpPost 简化的 HTTP POST 请求
func httpPost(url, contentType string, body io.Reader, header map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	client := &http.Client{}
	return client.Do(req)
}

// httpPut 简化的 HTTP PUT 请求
func httpPut(url, contentType string, body io.Reader, header map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("PUT", url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	client := &http.Client{}
	return client.Do(req)
}

// readClose 读取并关闭响应体
func readClose(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// ---------- 文件名处理 ----------

// ensureExt 确保文件名有正确的扩展名
func ensureExt(filename string, data []byte) string {
	ext := detectExt(data)
	if strings.HasSuffix(strings.ToLower(filename), "."+ext) {
		return filename
	}
	// 去掉旧后缀
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		filename = filename[:idx]
	}
	return filename + "." + ext
}

// ---------- 图床实现占位 ----------

// 各 tryXxx 函数在对应 .go 文件中实现
