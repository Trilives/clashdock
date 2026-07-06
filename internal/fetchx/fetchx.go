// Package fetchx 下载通道（对应 core.py 的 _Fetcher，net/http 取代 curl 子进程）：
// 「直连优先 → 代理兜底」。直连可达即彻底绕过环境代理（显式 Proxy=nil），
// 避免下载被静默隧道进本地 mihomo → 机场节点（慢）；直连不可达才用配置的代理。
package fetchx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Trilives/clashdock/internal/execx"
	"github.com/Trilives/clashdock/internal/i18n"
)

const ProbeURL = "https://www.google.com/generate_204"

const (
	retryAttempts = 3
	retryDelay    = 2 * time.Second
)

type Fetcher struct {
	Proxy    string // 兜底代理（直连不可用时才走）
	Token    string // GitHub Token（仅 ReadJSON 的 API 请求附带）
	directOK *bool

	ordered []string // 非空时 do() 按此顺序尝试（空串=直连），跳过直连探测/兜底逻辑
}

func New(proxy, token string) *Fetcher {
	return &Fetcher{Proxy: proxy, Token: token}
}

// NewOrdered 按给定顺序依次尝试代理候选（空字符串会被跳过），最后总以直连兜底；
// 不做直连可达性探测——调用方已经决定了"优先走代理"，不需要 New() 那套
// 直连优先、不可达才代理的判断。
func NewOrdered(candidates []string, token string) *Fetcher {
	ordered := make([]string, 0, len(candidates)+1)
	for _, c := range candidates {
		if strings.TrimSpace(c) != "" {
			ordered = append(ordered, c)
		}
	}
	ordered = append(ordered, "") // 直连兜底
	return &Fetcher{Token: token, ordered: ordered}
}

// clientFor 构造指定代理的 client（proxy 为空即 direct，显式绕过环境代理）。
func clientFor(proxy string) *http.Client {
	tr := &http.Transport{
		Proxy:                 nil, // direct：绕过 http_proxy 等环境变量
		ResponseHeaderTimeout: 30 * time.Second,
	}
	if proxy != "" {
		if u, err := url.Parse(proxy); err == nil {
			tr.Proxy = http.ProxyURL(u)
		}
	}
	return &http.Client{Transport: tr}
}

// DirectReachable 探测直连是否可用（结果缓存于本 Fetcher 生命周期）。
func (f *Fetcher) DirectReachable() bool {
	if f.directOK == nil {
		c := clientFor("")
		c.Timeout = 10 * time.Second
		resp, err := c.Get(ProbeURL)
		ok := err == nil && resp.StatusCode < 400
		if resp != nil {
			resp.Body.Close()
		}
		f.directOK = &ok
		if ok {
			execx.Info(i18n.T("直连可达，跳过代理。"))
		}
	}
	return *f.directOK
}

// attempts 返回本次 do() 依次尝试的代理候选（""=直连）。ordered 非空时按其顺序
// 尝试（NewOrdered 构造，跳过直连探测）；否则沿用直连优先、不可达才代理兜底。
func (f *Fetcher) attempts() []string {
	if f.ordered != nil {
		return f.ordered
	}
	noProxy := os.Getenv("MIHOMO_NO_PROXY") == "1"
	if f.Proxy != "" && !noProxy && !f.DirectReachable() {
		return []string{f.Proxy, ""}
	}
	return []string{""}
}

// do 按候选顺序执行 fn，首个成功即返回；全失败返回最后一个错误。
func (f *Fetcher) do(fn func(c *http.Client) error) error {
	tries := f.attempts()
	var last error
	for i, proxy := range tries {
		err := withRetry(func() error { return fn(clientFor(proxy)) })
		if err == nil {
			return nil
		}
		last = err
		if i < len(tries)-1 {
			label := proxy
			if label == "" {
				label = i18n.T("直连")
			}
			execx.Warn(fmt.Sprintf(i18n.T("  %s 失败（%v），改下一通道重试…"), label, err))
		}
	}
	return last
}

func withRetry(fn func() error) error {
	var err error
	for i := 0; i < retryAttempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < retryAttempts-1 {
			time.Sleep(retryDelay)
		}
	}
	return err
}

// ReadJSON 拉取 URL 并解码 JSON（用于 GitHub API；附带 Token）。
func (f *Fetcher) ReadJSON(rawURL string, v any) error {
	return f.do(func(c *http.Client) error {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "clashdock")
		req.Header.Set("Accept", "application/vnd.github+json")
		if f.Token != "" {
			req.Header.Set("Authorization", "Bearer "+f.Token)
		}
		resp, err := c.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, rawURL)
		}
		return json.NewDecoder(resp.Body).Decode(v)
	})
}

// FetchFile 下载到 path（支持 .part 断点续传语义：path 已有内容则尝试 Range 续传）。
// 不做完整性校验、不改名——由调用方（kernel）校验后落位。
func (f *Fetcher) FetchFile(rawURL, path string) error {
	return f.do(func(c *http.Client) error {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "clashdock")
		var offset int64
		if st, err := os.Stat(path); err == nil && st.Size() > 0 {
			offset = st.Size()
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
		resp, err := c.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var out *os.File
		switch {
		case resp.StatusCode == http.StatusPartialContent && offset > 0:
			out, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
		case resp.StatusCode < 400:
			out, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		default:
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, rawURL)
		}
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, resp.Body); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}
