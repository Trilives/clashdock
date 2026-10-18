package usermode

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// TunCapsSpec 用户模式开启 TUN 时经 sudo setcap 授予内核文件的能力：建 TUN 设备与改路由
// （CAP_NET_ADMIN）、原始套接字（CAP_NET_RAW）、绑定低端口（CAP_NET_BIND_SERVICE，DNS 劫持用）。
// 文件能力随文件内容走：内核更新（临时文件 + 改名）后新文件不带能力，需要重新授权。
const TunCapsSpec = "cap_net_admin,cap_net_bind_service,cap_net_raw+ep"

// 内核能力位与 vfs_cap_data 头部常量（linux/capability.h）。
const (
	capNetBindService = 10
	capNetAdmin       = 12
	capNetRaw         = 13

	capXattr          = "security.capability"
	vfsCapRevisionV1  = 0x01000000
	vfsCapRevisionMax = 0x03000000
	vfsCapRevMask     = 0xFF000000
	vfsCapFlagsEff    = 0x000001
)

// HasTunCaps 内核文件是否已带 TUN 所需的全部能力（permitted 含三项且置有效位）。
// 直接读 security.capability 扩展属性，不依赖 getcap。
func HasTunCaps(bin string) bool {
	buf := make([]byte, 64)
	n, err := syscall.Getxattr(bin, capXattr, buf)
	if err != nil || n <= 0 {
		return false
	}
	return parseTunCaps(buf[:n])
}

// parseTunCaps 解析 vfs_cap_data：magic_etc(u32) 后为 permitted/inheritable 低 32 位，
// v2/v3 另有高 32 位与 rootid。所需能力位都在低 32 位内。
func parseTunCaps(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	rev := magic & vfsCapRevMask
	if rev < vfsCapRevisionV1 || rev > vfsCapRevisionMax || magic&vfsCapFlagsEff == 0 {
		return false
	}
	permitted := binary.LittleEndian.Uint32(data[4:8])
	for _, bit := range []uint{capNetAdmin, capNetRaw, capNetBindService} {
		if permitted&(1<<bit) == 0 {
			return false
		}
	}
	return true
}

// sysClassNet 网络设备目录；测试替换为临时目录。
var sysClassNet = "/sys/class/net"

// WaitTunDevice 在 timeout 内轮询 TUN 设备是否出现，用于确认内核真的拿到了能力
// （家目录挂载为 nosuid 或在 NFS 上时 setcap 可能成功但能力不生效）。
func WaitTunDevice(name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(filepath.Join(sysClassNet, name)); err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}
