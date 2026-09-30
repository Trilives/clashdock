package usermode

import (
	"net"
	"strconv"
)

// PortBusy 本机 TCP 端口是否已被占用（试探监听 127.0.0.1:port）。用户模式常见于多人
// 共用的机器，默认的 7890 / 9090 可能已被别人的代理占着，部署前据此提醒改端口。
func PortBusy(port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

// BusyPorts 过滤出已被占用的端口（保持输入顺序）。
func BusyPorts(ports ...int) []int {
	var busy []int
	for _, p := range ports {
		if PortBusy(p) {
			busy = append(busy, p)
		}
	}
	return busy
}
