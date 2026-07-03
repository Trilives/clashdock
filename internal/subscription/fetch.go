// 拉取订阅原始内容（对应 fetch.py，net/http 取代 curl）。
//
// 机场常按 User-Agent 决定返回的订阅格式，故按来源类型设置合适的 UA。
// 可选经局域网 download_proxy 下载（覆盖出海慢的机场）。
package subscription

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// clash 用 mihomo UA，促使机场返回 Clash.Meta 专属字段（如 hysteria2/vless reality）
var userAgents = map[string]string{
	"clash":  "mihomo/1.19.0 clash-verge/v2.0.0",
	"base64": "v2rayN/6.0",
}

const (
	fetchRetries    = 3
	fetchRetryDelay = 2 * time.Second
	fetchTimeout    = 120 * time.Second
)

// Fetch 下载订阅内容，返回原始字节。
func Fetch(rawURL, sourceType, proxy string) ([]byte, error) {
	ua, ok := userAgents[sourceType]
	if !ok {
		ua = "Mozilla/5.0"
	}
	tr := &http.Transport{Proxy: nil}
	if proxy != "" {
		if u, err := url.Parse(proxy); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	client := &http.Client{Transport: tr, Timeout: fetchTimeout}

	var lastErr error
	for i := 0; i < fetchRetries; i++ {
		if i > 0 {
			time.Sleep(fetchRetryDelay)
		}
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", ua)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 400 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, rawURL)
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		return data, nil
	}
	return nil, fmt.Errorf("拉取订阅失败: %w", lastErr)
}
