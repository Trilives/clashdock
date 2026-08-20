package clashapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurrentSelectionsParsesPolicyGroups(t *testing.T) {
	type requestSnapshot struct {
		method string
		path   string
		auth   string
	}
	requestSeen := make(chan requestSnapshot, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSeen <- requestSnapshot{r.Method, r.URL.Path, r.Header.Get("Authorization")}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "proxies": {
    "手动选择": {"type": "Selector", "now": "香港 01", "all": ["香港 01", "日本 01"]},
    "自动选择": {"type": "URLTest", "now": "日本 01", "all": ["香港 01", "日本 01"]},
    "香港 01": {"type": "Shadowsocks", "udp": true}
  }
}`))
	}))
	defer server.Close()

	client := &Client{Base: server.URL, secret: "test-secret", http: server.Client()}
	got, err := client.CurrentSelections()
	if err != nil {
		t.Fatalf("CurrentSelections() error = %v", err)
	}
	request := <-requestSeen
	if request.method != http.MethodGet || request.path != "/proxies" {
		t.Fatalf("请求 = %s %s，期望 GET /proxies", request.method, request.path)
	}
	if request.auth != "Bearer test-secret" {
		t.Fatalf("Authorization = %q", request.auth)
	}
	want := map[string]string{
		"手动选择": "香港 01",
		"自动选择": "日本 01",
	}
	if len(got) != len(want) {
		t.Fatalf("CurrentSelections() = %v，期望 %v", got, want)
	}
	for group, node := range want {
		if got[group] != node {
			t.Errorf("CurrentSelections()[%q] = %q，期望 %q", group, got[group], node)
		}
	}
}

func TestCurrentSelectionsRejectsNonSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := &Client{Base: server.URL, http: server.Client()}
	if _, err := client.CurrentSelections(); err == nil {
		t.Fatal("CurrentSelections() 在 GET /proxies 返回非 2xx 时应返回错误")
	}
}
