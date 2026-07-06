// 拉取订阅原始内容（对应 fetch.py，net/http 取代 curl）。
//
// 机场常按 User-Agent 决定返回的订阅格式，故按来源类型设置合适的 UA。
// 可按序尝试多个代理候选（本机 mixed-port / download_proxy），空字符串候选
// 代表直连；直连是"尽力而为"的绕过——TUN 模式下仍可能被路由劫持，调用方
// （subscription.manager）按需另行暂停服务以确保真正直连。
package subscription

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
)

// clash 用 mihomo UA，促使机场返回 Clash.Meta 专属字段（如 hysteria2/vless reality）
var userAgents = map[string]string{
	"clash":  "mihomo/1.19.0 clash-verge/v2.0.0",
	"base64": "v2rayN/6.0",
}

const (
	fetchRetries     = 3
	fetchRetryDelay  = 2 * time.Second
	fetchDialTimeout = 10 * time.Second
	// 单次尝试超时：链接/代理不可达时应快速失败并重试，而不是让用户等一次
	// 就长达数分钟——之前 120s×3 次的组合在代理不通时会让整个操作看起来像
	// 卡死。绝大多数订阅原文体积很小，30s 对正常网络绰绰有余。
	fetchTimeout = 30 * time.Second
)

// Fetch 下载订阅内容，返回原始字节。proxies 为按序尝试的代理候选（空字符串=
// 直连）；nil/空切片等价于只直连一次。每个候选各自重试 fetchRetries 次，
// 耗尽后才换下一个候选。
func Fetch(rawURL, sourceType string, proxies []string) ([]byte, error) {
	ua, ok := userAgents[sourceType]
	if !ok {
		ua = "Mozilla/5.0"
	}
	if len(proxies) == 0 {
		proxies = []string{""}
	}

	var lastErr error
	for pi, proxy := range proxies {
		data, err := fetchOnce(rawURL, ua, proxy)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if pi < len(proxies)-1 {
			execx.Warn(fmt.Sprintf(i18n.T("  代理候选失败（%v），改下一候选重试…"), err))
		}
	}
	return nil, fmt.Errorf(i18n.T("拉取订阅失败: %w"), lastErr)
}

// fetchOnce 用单个代理候选（空=直连）拉取，内部重试 fetchRetries 次。
func fetchOnce(rawURL, ua, proxy string) ([]byte, error) {
	tr := &http.Transport{
		Proxy:               nil,
		DialContext:         (&net.Dialer{Timeout: fetchDialTimeout}).DialContext,
		TLSHandshakeTimeout: fetchDialTimeout,
	}
	if proxy != "" {
		if u, err := url.Parse(proxy); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	client := &http.Client{Transport: tr, Timeout: fetchTimeout}

	var lastErr error
	for i := 0; i < fetchRetries; i++ {
		if i > 0 {
			execx.Warn(fmt.Sprintf(i18n.T("拉取失败（%v），第 %d/%d 次重试…"), lastErr, i+1, fetchRetries))
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
	return nil, lastErr
}
