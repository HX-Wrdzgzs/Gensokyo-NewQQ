// Package imagehosting 提供统一的图床上传接口。
//
// 支持 6 种可用图床后端，按配置顺序依次尝试，第一个成功的返回结果：
//  1. COS (腾讯云对象存储)
//  2. Bilibili (B站图床)
//  3. QQ频道 (通过发消息获取 qpic.cn 链接)
//  4. ChatGLM (需显式允许第三方免配置图床)
//  5. Ukaka (需显式允许第三方免配置图床)
//  6. 星野 (需显式允许第三方免配置图床)
//
// Nature 后端因上游实现包含公开的内置访问凭据而被禁用。
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
	"os"
	"path"
	"strings"
	"time"

	"github.com/hoshinonyaruko/gensokyo/config"
	"github.com/hoshinonyaruko/gensokyo/mylog"
)

const (
	maxImageBytes        = 10 << 20 // 10 MiB
	maxImagePixels int64 = 40_000_000
	maxResponseBodyBytes = 1 << 20 // 1 MiB
)

var imageHostingHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

// Upload 解码 base64 图片后按配置优先级依次尝试各图床。
// 返回公开可访问的图片 URL。
func Upload(base64Data string, filename string) (string, error) {
	// 在分配解码缓冲区前先拒绝明显超限的输入。
	if len(base64Data) > base64.StdEncoding.EncodedLen(maxImageBytes)+4 {
		return "", fmt.Errorf("图片超过最大限制 %d MiB", maxImageBytes>>20)
	}

	imageBytes, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("base64 解码失败: %w", err)
	}
	return uploadImageData(imageBytes, filename)
}

// UploadBytes 直接上传 bytes（无需 base64 编解码）。
func UploadBytes(imageData []byte, filename string) (string, error) {
	return uploadImageData(imageData, filename)
}

func uploadImageData(imageData []byte, filename string) (string, error) {
	if err := validateImageData(imageData); err != nil {
		return "", err
	}
	filename = ensureExt(filename, imageData)

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
	}

	var lastErr error
	for _, uploader := range uploaders {
		if !isEnabled(uploader.name) {
			continue
		}
		url, err := uploader.fn(imageData, filename)
		if err == nil && url != "" {
			mylog.Printf("图床 [%s] 上传成功", uploader.name)
			return url, nil
		}
		if err != nil {
			lastErr = err
			mylog.Printf("图床 [%s] 上传失败: %v", uploader.name, err)
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf("所有已启用图床均上传失败: %w", lastErr)
	}
	return "", fmt.Errorf("没有可用的图床后端")
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
		return thirdPartyImageHostsAllowed() && config.GetImageHostingChatGLM().Enabled
	case "Ukaka":
		return thirdPartyImageHostsAllowed() && config.GetImageHostingUkaka().Enabled
	case "Xingye":
		return thirdPartyImageHostsAllowed() && config.GetImageHostingXingye().Enabled
	}
	return false
}

// thirdPartyImageHostsAllowed 要求管理员明确允许无需凭据的第三方上传服务。
// 这可以保护仍沿用旧版 enabled: true 配置的用户，避免升级后静默外传图片。
func thirdPartyImageHostsAllowed() bool {
	value := strings.TrimSpace(os.Getenv("GENSOKYO_ENABLE_THIRD_PARTY_IMAGE_HOSTS"))
	return value == "1" || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}

// ---------- 输入校验 ----------

func validateImageData(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("图片数据为空")
	}
	if len(data) > maxImageBytes {
		return fmt.Errorf("图片超过最大限制 %d MiB", maxImageBytes>>20)
	}

	mimeType := detectMIME(data)
	if mimeType == "" {
		return fmt.Errorf("不支持或无法识别的图片格式")
	}

	// 标准库没有内置 WebP 解码器，因此 WebP 仅执行文件头检查。
	if mimeType != "image/webp" {
		width, height := getImageDimensions(data)
		if width <= 0 || height <= 0 {
			return fmt.Errorf("图片内容损坏或无法解析")
		}
		if int64(width)*int64(height) > maxImagePixels {
			return fmt.Errorf("图片像素数量超过限制")
		}
	}
	return nil
}

// detectMIME 从图片 bytes 检测 MIME 类型；无法识别时返回空字符串。
func detectMIME(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	case len(data) >= 3 && bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case len(data) >= 6 && (bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))):
		return "image/gif"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	default:
		return ""
	}
}

// detectExt 从图片 bytes 检测扩展名。
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
		return ""
	}
}

// getImageDimensions 从 bytes 读取图片尺寸。
func getImageDimensions(data []byte) (int, int) {
	img, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return img.Width, img.Height
}

// ---------- HTTP 辅助函数 ----------

func httpPost(url, contentType string, body io.Reader, header map[string]string) (*http.Response, error) {
	return doImageHostingRequest(http.MethodPost, url, contentType, body, header)
}

func httpPut(url, contentType string, body io.Reader, header map[string]string) (*http.Response, error) {
	return doImageHostingRequest(http.MethodPut, url, contentType, body, header)
}

func doImageHostingRequest(method, url, contentType string, body io.Reader, header map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range header {
		req.Header.Set(key, value)
	}
	req.Header.Set("User-Agent", "Gensokyo-NewQQ/imagehosting")
	return imageHostingHTTPClient.Do(req)
}

// readClose 读取并关闭响应体，同时限制响应体大小。
func readClose(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("HTTP 响应为空")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxResponseBodyBytes {
		return nil, fmt.Errorf("HTTP 响应体超过 %d MiB", maxResponseBodyBytes>>20)
	}
	return body, nil
}

// ---------- 文件名处理 ----------

// ensureExt 清理文件名并确保扩展名与实际图片格式一致。
func ensureExt(filename string, data []byte) string {
	normalized := strings.ReplaceAll(filename, "\\", "/")
	filename = path.Base(normalized)
	filename = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, filename)
	filename = strings.TrimSpace(filename)
	if filename == "" || filename == "." || filename == ".." {
		filename = "image"
	}

	if index := strings.LastIndex(filename, "."); index > 0 {
		filename = filename[:index]
	}
	filename = strings.Trim(filename, ". ")
	if filename == "" {
		filename = "image"
	}

	ext := detectExt(data)
	if ext == "" {
		return filename
	}
	return filename + "." + ext
}
