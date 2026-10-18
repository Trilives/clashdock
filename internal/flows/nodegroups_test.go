package flows

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Trilives/clashdock/internal/config"
)

func TestPickGroupDoesNotGuessLargestSelectWhenMainGroupIsUnrecognized(t *testing.T) {
	cfg := nodeSelectConfig(
		nodeSelectGroup("入口", "香港 01"),
		nodeSelectGroup("机场线路", "日本 01", "新加坡 01", "美国 01"),
	)

	group, err := pickGroup(cfg, "", []string{"主选择", "proxy"})
	if err == nil {
		t.Fatalf("pickGroup() = %v, nil；未命中主选择组时不应猜测成员最多的 select 组", group)
	}
	if group != nil {
		t.Fatalf("pickGroup() group = %v, want nil", group)
	}
	if !strings.Contains(err.Error(), "未识别到主选择组") {
		t.Fatalf("pickGroup() error = %q，应可判定为未识别主选择组", err)
	}
}

func TestResolveMainGroupPrependsEnteredKeywordPersistsAndRecognizesImmediately(t *testing.T) {
	p := prepareMainGroupCustomize(t, []string{"旧关键词"})
	cfg := nodeSelectConfig(
		nodeSelectGroup("Alpha Control", "香港 01", "日本 01"),
		nodeSelectGroup("Other Pool", "新加坡 01"),
	)

	inputCalls := 0
	group, err := resolveMainGroup(p, cfg, "", func(prompt string) (string, error) {
		inputCalls++
		if !strings.Contains(prompt, "主选择组") {
			t.Fatalf("输入提示 = %q，应说明主选择组", prompt)
		}
		return "Alpha", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if inputCalls != 1 {
		t.Fatalf("input 调用次数 = %d, want 1", inputCalls)
	}
	if got := group["name"]; got != "Alpha Control" {
		t.Fatalf("resolveMainGroup() name = %v, want Alpha Control", got)
	}
	if got := config.StrList(config.Load(p), "main_group_keywords"); len(got) < 2 || got[0] != "Alpha" || got[1] != "旧关键词" {
		t.Fatalf("保存后的 main_group_keywords = %v, want [Alpha 旧关键词 ...]", got)
	}
}

func TestResolveMainGroupEmptyInputDoesNotWriteCustomize(t *testing.T) {
	p := prepareMainGroupCustomize(t, []string{"旧关键词"})
	before, err := os.ReadFile(p.CustomizeFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg := nodeSelectConfig(nodeSelectGroup("Unknown Pool", "香港 01"))

	group, err := resolveMainGroup(p, cfg, "", func(string) (string, error) {
		return "  \t ", nil
	})
	if err == nil || !strings.Contains(err.Error(), "未识别到主选择组，无法切换") {
		t.Fatalf("resolveMainGroup() = %v, %v；空输入应返回无法切换提示", group, err)
	}
	if group != nil {
		t.Fatalf("resolveMainGroup() group = %v, want nil", group)
	}
	after, readErr := os.ReadFile(p.CustomizeFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("空输入后 customize.json 被改写\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestResolveMainGroupPropagatesInputErrorWithoutWritingCustomize(t *testing.T) {
	p := prepareMainGroupCustomize(t, []string{"旧关键词"})
	before, err := os.ReadFile(p.CustomizeFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg := nodeSelectConfig(nodeSelectGroup("Unknown Pool", "香港 01"))
	wantErr := errors.New("input cancelled")

	group, err := resolveMainGroup(p, cfg, "", func(string) (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("resolveMainGroup() error = %v, want wrapped %v", err, wantErr)
	}
	if group != nil {
		t.Fatalf("resolveMainGroup() group = %v, want nil", group)
	}
	after, readErr := os.ReadFile(p.CustomizeFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("输入失败后 customize.json 被改写\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestResolveMainGroupRejectsUnmatchedKeywordWithoutWritingCustomize(t *testing.T) {
	p := prepareMainGroupCustomize(t, []string{"旧关键词"})
	before, err := os.ReadFile(p.CustomizeFile)
	if err != nil {
		t.Fatal(err)
	}
	cfg := nodeSelectConfig(nodeSelectGroup("Unknown Pool", "香港 01"))

	group, err := resolveMainGroup(p, cfg, "", func(string) (string, error) {
		return "不存在的关键词", nil
	})
	if err == nil || !strings.Contains(err.Error(), "未匹配") {
		t.Fatalf("resolveMainGroup() = %v, %v；无效关键词应返回未匹配提示", group, err)
	}
	if group != nil {
		t.Fatalf("resolveMainGroup() group = %v, want nil", group)
	}
	after, readErr := os.ReadFile(p.CustomizeFile)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(before) {
		t.Fatalf("无效关键词输入后 customize.json 被改写\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestCollectMembersBucketsNodesAndSeparatesSubgroups(t *testing.T) {
	cfg := nodeSelectConfig(
		nodeSelectGroup("Proxy", "Auto", "DIRECT", "剩余流量：10G", "香港 01", "Japan 02", "Germany 03"),
		map[string]any{"name": "Auto", "type": "url-test", "proxies": []any{"香港 01"}},
	)
	groups := groupsOf(cfg)

	buckets, subgroups := collectMembers(cfg, groups[0])

	if !reflect.DeepEqual(subgroups, []string{"Auto"}) {
		t.Fatalf("subgroups = %v", subgroups)
	}
	want := map[string][]string{"hk": {"香港 01"}, "jp": {"Japan 02"}, otherKey: {"Germany 03"}}
	if !reflect.DeepEqual(buckets, want) {
		t.Fatalf("buckets = %v, want %v", buckets, want)
	}
}
