// Package configfile reads mihomo/Clash config files.
package configfile

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Parse(raw []byte) (map[string]any, error) {
	var cfg map[string]any
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("配置根节点不是映射")
	}
	return cfg, nil
}

func Read(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("解析配置 %s: %w", path, err)
	}
	return cfg, nil
}
