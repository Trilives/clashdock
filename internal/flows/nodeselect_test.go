package flows

import (
	"strings"
	"testing"
)

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
