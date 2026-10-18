package flows

import (
	"strings"
	"testing"
)

func TestCurrentNodeSummaryOnlyIncludesRecognizedMainGroup(t *testing.T) {
	cfg := nodeSelectConfig(
		nodeSelectGroup("Main Select", "香港 01", "日本 01"),
		nodeSelectGroup("Fallback Select", "新加坡 01", "美国 01"),
	)
	selections := map[string]string{
		"Main Select":     "日本 01",
		"Fallback Select": "美国 01",
	}

	lines := currentNodeSummary(cfg, "Main Select", selections, true)
	if len(lines) != 1 {
		t.Fatalf("currentNodeSummary() = %v, want exactly one main-group line", lines)
	}
	if !strings.Contains(lines[0], "Main Select") || !strings.Contains(lines[0], "日本 01") || !strings.Contains(lines[0], "当前运行") {
		t.Fatalf("主选择组当前状态 = %q，缺少组名、当前节点或运行态", lines[0])
	}
	if strings.Contains(lines[0], "Fallback Select") || strings.Contains(lines[0], "美国 01") {
		t.Fatalf("主选择组摘要不应包含其它组状态：%q", lines[0])
	}
}

func TestCurrentNodeSummaryFallsBackToMainGroupConfiguredPreference(t *testing.T) {
	cfg := nodeSelectConfig(
		nodeSelectGroup("Main Select", "香港 01", "日本 01"),
		nodeSelectGroup("Fallback Select", "美国 01"),
	)

	lines := currentNodeSummary(cfg, "Main Select", nil, false)
	if len(lines) != 1 {
		t.Fatalf("currentNodeSummary() = %v, want exactly one main-group line", lines)
	}
	if !strings.Contains(lines[0], "Main Select") || !strings.Contains(lines[0], "香港 01") ||
		!strings.Contains(lines[0], "配置首选") || !strings.Contains(lines[0], "非运行时状态") {
		t.Fatalf("主选择组配置状态 = %q，缺少组名、配置首选或非运行时标记", lines[0])
	}
	if strings.Contains(lines[0], "Fallback Select") || strings.Contains(lines[0], "美国 01") {
		t.Fatalf("主选择组摘要不应包含其它组状态：%q", lines[0])
	}
}

func TestCurrentNodeSummaryOmitsUnavailableOrUnknownMainGroupState(t *testing.T) {
	tests := []struct {
		name        string
		cfg         map[string]any
		target      string
		selections  map[string]string
		runtimeOkay bool
	}{
		{
			name:        "runtime response lacks main group",
			cfg:         nodeSelectConfig(nodeSelectGroup("Main Select", "香港 01")),
			target:      "Main Select",
			selections:  map[string]string{"Other Select": "日本 01"},
			runtimeOkay: true,
		},
		{
			name:        "configured group has no preferred member",
			cfg:         nodeSelectConfig(nodeSelectGroup("Main Select")),
			target:      "Main Select",
			runtimeOkay: false,
		},
		{
			name:        "target group is absent",
			cfg:         nodeSelectConfig(nodeSelectGroup("Other Select", "香港 01")),
			target:      "Main Select",
			runtimeOkay: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if lines := currentNodeSummary(tt.cfg, tt.target, tt.selections, tt.runtimeOkay); len(lines) != 0 {
				t.Fatalf("currentNodeSummary() = %v, want no misleading state", lines)
			}
		})
	}
}

func TestCurrentNodeLabelsDistinguishRuntimeAndConfiguredPreferred(t *testing.T) {
	names := []string{"香港 01", "日本 01", "新加坡 01"}
	labels := currentNodeLabels(
		names,
		map[string]int{"香港 01": 38, "日本 01": 61, "新加坡 01": 75},
		"香港 01",
		"日本 01",
		true,
	)

	runtimeLabel := nodeLabel(t, names, labels, "香港 01")
	if !strings.Contains(runtimeLabel, "当前运行") {
		t.Fatalf("运行时当前节点标签 = %q，应包含“当前运行”", runtimeLabel)
	}
	if strings.Contains(runtimeLabel, "配置首选") {
		t.Fatalf("仅为运行时当前节点的标签 = %q，不应标成配置首选", runtimeLabel)
	}

	configuredLabel := nodeLabel(t, names, labels, "日本 01")
	if !strings.Contains(configuredLabel, "配置首选") {
		t.Fatalf("配置固定首选节点标签 = %q，应包含“配置首选”", configuredLabel)
	}
	if strings.Contains(configuredLabel, "当前运行") {
		t.Fatalf("仅为配置首选节点的标签 = %q，不应标成当前运行", configuredLabel)
	}
}

func TestCurrentNodeLabelsCanShowRuntimeAndConfiguredOnSameNode(t *testing.T) {
	names := []string{"香港 01"}
	labels := currentNodeLabels(names, map[string]int{"香港 01": 38}, "香港 01", "香港 01", true)
	label := nodeLabel(t, names, labels, "香港 01")

	if !strings.Contains(label, "当前运行") || !strings.Contains(label, "配置首选") {
		t.Fatalf("同为运行时当前与配置首选的节点标签 = %q，应同时显示两种状态", label)
	}
}

func TestCurrentNodeLabelsMarksConfiguredStateAsNonRuntimeWhenAPIUnavailable(t *testing.T) {
	names := []string{"香港 01", "日本 01"}
	labels := currentNodeLabels(names, nil, "", "日本 01", false)
	configuredLabel := nodeLabel(t, names, labels, "日本 01")

	if !strings.Contains(configuredLabel, "配置首选") || !strings.Contains(configuredLabel, "非运行时状态") {
		t.Fatalf("API 不可用时的配置首选标签 = %q，应明确标注配置态且非运行时", configuredLabel)
	}
	if strings.Contains(configuredLabel, "当前运行") {
		t.Fatalf("API 不可用时的配置首选标签 = %q，不应冒充当前运行状态", configuredLabel)
	}
}

func nodeLabel(t *testing.T, names, labels []string, node string) string {
	t.Helper()
	if len(labels) != len(names) {
		t.Fatalf("标签数量 = %d，节点数量 = %d", len(labels), len(names))
	}
	for i, name := range names {
		if name == node {
			return labels[i]
		}
	}
	t.Fatalf("测试节点 %q 不在节点列表中", node)
	return ""
}
