// Package securityaudit inspects deployment configuration for high-risk network defaults.
package securityaudit

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Severity represents the operational impact of a finding.
type Severity string

const (
	SeverityWarning Severity = "warning"
	SeverityHigh    Severity = "high"
)

// Finding describes one insecure or ambiguous deployment setting.
type Finding struct {
	Severity Severity
	Code     string
	Message  string
}

// Report is the result of auditing one config.yml file.
type Report struct {
	Findings []Finding
}

func (r Report) HasHighRisk() bool {
	for _, finding := range r.Findings {
		if finding.Severity == SeverityHigh {
			return true
		}
	}
	return false
}

type configFile struct {
	Settings struct {
		Port               string `yaml:"port"`
		ForceSSL           bool   `yaml:"force_ssl"`
		EnableWSServer     bool   `yaml:"enable_ws_server"`
		WSServerToken      string `yaml:"ws_server_token"`
		HTTPAddress        string `yaml:"http_address"`
		HTTPAccessToken    string `yaml:"http_access_token"`
		DisableWebUI       bool   `yaml:"disable_webui"`
		WebUIUsername      string `yaml:"server_user_name"`
		WebUIPassword      string `yaml:"server_user_password"`
		ThirdPartyImageOpt struct {
			ChatGLM struct {
				Enabled bool `yaml:"enabled"`
			} `yaml:"chatglm"`
			Ukaka struct {
				Enabled bool `yaml:"enabled"`
			} `yaml:"ukaka"`
			Xingye struct {
				Enabled bool `yaml:"enabled"`
			} `yaml:"xingye"`
			Nature struct {
				Enabled bool `yaml:"enabled"`
			} `yaml:"nature"`
		} `yaml:"image_hosting"`
	} `yaml:"settings"`
}

// AuditFile reads and audits a Gensokyo config file.
func AuditFile(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	return AuditYAML(data)
}

// AuditYAML audits config bytes without modifying them.
func AuditYAML(data []byte) (Report, error) {
	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Report{}, fmt.Errorf("parse config for security audit: %w", err)
	}

	settings := cfg.Settings
	report := Report{}
	add := func(severity Severity, code, message string) {
		report.Findings = append(report.Findings, Finding{Severity: severity, Code: code, Message: message})
	}

	if settings.EnableWSServer && strings.TrimSpace(settings.WSServerToken) == "" {
		add(SeverityHigh, "ws-empty-token", "正向 WebSocket 监听在全部网络接口，但 ws_server_token 为空")
	}

	if address := strings.TrimSpace(settings.HTTPAddress); address != "" {
		if strings.TrimSpace(settings.HTTPAccessToken) == "" {
			if IsLoopbackAddress(address) {
				add(SeverityWarning, "http-api-loopback-empty-token", "HTTP API 仅监听本机但未配置 http_access_token")
			} else {
				add(SeverityHigh, "http-api-public-empty-token", "HTTP API 可能对外监听，但 http_access_token 为空")
			}
		} else {
			add(SeverityWarning, "http-api-query-token", "HTTP API 已配置令牌；调用方应只使用 Authorization: Bearer，避免把令牌放入 URL 查询参数")
		}
	}

	if !settings.DisableWebUI {
		username := strings.TrimSpace(settings.WebUIUsername)
		password := settings.WebUIPassword
		if username == "" || password == "" {
			add(SeverityHigh, "webui-empty-credentials", "WebUI 已启用，但用户名或密码为空")
		} else if isKnownDefaultCredential(username, password) {
			add(SeverityHigh, "webui-default-credentials", "WebUI 仍在使用模板默认凭据，请立即修改")
		} else if len([]rune(password)) < 12 {
			add(SeverityWarning, "webui-weak-password", "WebUI 密码少于 12 个字符")
		}
		if !settings.ForceSSL && strings.TrimSpace(settings.Port) != "443" {
			add(SeverityWarning, "webui-plaintext", "WebUI 主服务未启用 HTTPS；不要直接暴露到不可信网络")
		}
	}

	thirdPartyConfigured := settings.ThirdPartyImageOpt.ChatGLM.Enabled ||
		settings.ThirdPartyImageOpt.Ukaka.Enabled ||
		settings.ThirdPartyImageOpt.Xingye.Enabled
	if thirdPartyConfigured && !ThirdPartyImageHostsOptedIn() {
		add(SeverityWarning, "third-party-image-hosts-gated", "配置中启用了第三方图床，但运行时显式授权未开启；图片不会上传到这些后端")
	}
	if settings.ThirdPartyImageOpt.Nature.Enabled {
		add(SeverityWarning, "nature-disabled", "Nature 图床配置仍为 enabled，但该后端已因公开凭据问题永久禁用")
	}

	return report, nil
}

// StrictModeEnabled reports whether startup should fail on high-risk findings.
func StrictModeEnabled() bool {
	return parseBoolEnv("GENSOKYO_STRICT_SECURITY")
}

// ThirdPartyImageHostsOptedIn mirrors the image-host runtime opt-in.
func ThirdPartyImageHostsOptedIn() bool {
	return parseBoolEnv("GENSOKYO_ENABLE_THIRD_PARTY_IMAGE_HOSTS")
}

func parseBoolEnv(name string) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if parsed, err := strconv.ParseBool(value); err == nil {
		return parsed
	}
	return strings.EqualFold(value, "yes") || strings.EqualFold(value, "on")
}

func isKnownDefaultCredential(username, password string) bool {
	username = strings.ToLower(strings.TrimSpace(username))
	return (username == "useradmin" && password == "admin") ||
		(username == "admin" && password == "admin")
}

// IsLoopbackAddress determines whether an HTTP listen address is constrained to localhost.
func IsLoopbackAddress(address string) bool {
	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		// Accept a bare host only for audit purposes.
		host = address
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
