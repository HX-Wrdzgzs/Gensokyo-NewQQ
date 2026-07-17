package main

import (
	"net"
	"net/http"
	"time"
)

const defaultOutboundHTTPTimeout = 60 * time.Second

// init 为仓库中直接使用 http.DefaultClient/http.DefaultTransport 的调用提供统一的超时基线。
// 具体业务如果需要更短或更长的超时，仍应创建独立 Client 并通过 context 控制生命周期。
func init() {
	configureDefaultHTTPClient()
}

func configureDefaultHTTPClient() {
	previousTransport := http.DefaultTransport
	transport, ok := previousTransport.(*http.Transport)
	if !ok {
		// 第三方代码若已替换默认 Transport，不覆盖其实现；至少保证默认 Client 有总超时。
		if http.DefaultClient.Timeout == 0 {
			http.DefaultClient.Timeout = defaultOutboundHTTPTimeout
		}
		return
	}

	cloned := transport.Clone()
	cloned.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	cloned.TLSHandshakeTimeout = 10 * time.Second
	cloned.ResponseHeaderTimeout = 20 * time.Second
	cloned.ExpectContinueTimeout = 1 * time.Second
	cloned.IdleConnTimeout = 90 * time.Second
	cloned.MaxIdleConns = 100
	cloned.MaxIdleConnsPerHost = 10

	http.DefaultTransport = cloned

	// 只在 DefaultClient 尚未绑定自定义 Transport 时接入新的默认 Transport，避免覆盖第三方配置。
	if http.DefaultClient.Transport == nil || http.DefaultClient.Transport == previousTransport {
		http.DefaultClient.Transport = cloned
	}
	if http.DefaultClient.Timeout == 0 {
		http.DefaultClient.Timeout = defaultOutboundHTTPTimeout
	}
}
