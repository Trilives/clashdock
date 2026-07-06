// 资源新鲜度标记：记录上一次完整部署（Install）时 state 侧资源文件的指纹，
// 供之后判断"内核/geo 数据是否下载了新版本，但运行时目录还没跟着更新"。
//
// 背景：SyncAndRestart 只重新同步 config.yaml，不会重新拷贝二进制/geo 文件
// （那样做对它的高频调用方——切换/固定节点、订阅刷新——太浪费）；只有完整的
// Install() 才会把 state/ 的资源真正部署到 /var/lib/clashdock-runtime。这里的
// 标记让调用方（cmd/clashdock 的交互式入口）能在启动时提醒用户"有更新但还没
// 生效，要不要重启应用"。
package sysd

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Trilives/clashdock/internal/paths"
)

type assetFingerprint struct {
	ModTime int64 `json:"mtime"`
	Size    int64 `json:"size"`
}

func deployedMarkerPath(p paths.Paths) string {
	return filepath.Join(p.State, "runtime-deployed.json")
}

func trackedAssetPaths(p paths.Paths) []string {
	return []string{p.MihomoBin, p.GeoipMetadb, p.CountryMmdb, p.GeositeDat}
}

func fingerprintAssets(p paths.Paths) map[string]assetFingerprint {
	out := make(map[string]assetFingerprint)
	for _, path := range trackedAssetPaths(p) {
		st, err := os.Stat(path)
		if err != nil {
			continue
		}
		out[path] = assetFingerprint{ModTime: st.ModTime().UnixNano(), Size: st.Size()}
	}
	return out
}

// RecordDeployedAssets 记录当前 state 侧资源文件的指纹；在一次成功的完整
// Install() 之后调用，供 AssetsStale 之后比对是否又发生了未部署的下载。
func RecordDeployedAssets(p paths.Paths) error {
	data, err := json.MarshalIndent(fingerprintAssets(p), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(deployedMarkerPath(p), data, 0o644)
}

// AssetsStale 判断 state 侧资源文件是否在上次记录的部署之后又被更新过（下载了
// 新内核/geo 数据，但运行时目录还没有重新部署）。没有标记文件（本功能上线前
// 就已存在的部署）视为不算陈旧，避免对老用户无端弹提示。
func AssetsStale(p paths.Paths) bool {
	raw, err := os.ReadFile(deployedMarkerPath(p))
	if err != nil {
		return false
	}
	var recorded map[string]assetFingerprint
	if err := json.Unmarshal(raw, &recorded); err != nil {
		return false
	}
	for path, fp := range fingerprintAssets(p) {
		if rec, ok := recorded[path]; !ok || rec != fp {
			return true
		}
	}
	return false
}
