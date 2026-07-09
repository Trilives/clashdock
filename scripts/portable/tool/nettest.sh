#!/usr/bin/env bash
# clashdock 便携包网络自测脚本（无需 root）。
#
# 面向便携/轻量模式：clashdock 只在本机提供 mixed 入站（127.0.0.1:<port>），
# 本脚本分别测「直连」与「经本地代理」两条路的连通性、时延与出口 IP，
# 帮你快速判断代理有没有真正生效、出海是否走了机场节点。
#
# 代理端口默认 7890；若便携工作目录里的 customize.json 改过 proxy_port 会自动读取。
# 可用 PORT=xxxx ./tool/nettest.sh 覆盖。
set -euo pipefail

TOOL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$TOOL_DIR/.." && pwd)"

command -v curl >/dev/null 2>&1 || { echo "错误：需要 curl。" >&2; exit 1; }

# resolve_port 优先取 PORT 环境变量，其次读便携工作目录 customize.json 的 proxy_port，兜底 7890。
resolve_port() {
	if [ -n "${PORT:-}" ]; then echo "$PORT"; return; fi
	local cfg="${CLASHDOCK_HOME:-$ROOT_DIR/clashdock-data}/customize.json"
	if [ -f "$cfg" ]; then
		local p
		p="$(grep -oE '"proxy_port"[[:space:]]*:[[:space:]]*[0-9]+' "$cfg" | grep -oE '[0-9]+' | head -n1)"
		if [ -n "$p" ]; then echo "$p"; return; fi
	fi
	echo 7890
}
PORT="$(resolve_port)"
PROXY="http://127.0.0.1:${PORT}"
PROBE="https://www.google.com/generate_204"
IP_URL="https://api.ip.sb/ip"

echo "本地代理：$PROXY"
echo

# probe 打印一条通道的连通性与耗时。$1=标签 $2=curl 额外参数（空或 -x 代理）。
probe() {
	local label="$1"; shift
	local code t out
	if out="$(curl -s -o /dev/null -m 10 -w '%{http_code} %{time_total}' "$@" "$PROBE" 2>/dev/null)"; then
		code="${out%% *}"; t="${out##* }"
		if [ "$code" = "204" ] || [ "$code" = "200" ]; then
			echo "  $label: 连通 (HTTP $code, ${t}s)"
			return 0
		fi
		echo "  $label: 异常 (HTTP $code)"
		return 1
	fi
	echo "  $label: 不通（超时或连接失败）"
	return 1
}

# egress_ip 打印一条通道看到的出口 IP。$@=curl 额外参数。
egress_ip() {
	local label="$1"; shift
	local ip
	ip="$(curl -s -m 10 "$@" "$IP_URL" 2>/dev/null | tr -d '[:space:]')"
	[ -n "$ip" ] && echo "  $label: $ip" || echo "  $label: 未知（取 IP 失败）"
}

echo "连通性 / 时延（$PROBE）："
probe "直连" || true
probe "经代理" -x "$PROXY" || true
echo

echo "出口 IP（$IP_URL）："
egress_ip "直连" || true
egress_ip "经代理" -x "$PROXY" || true
echo

echo "说明：两条「出口 IP」不同 = 代理已生效、出海走的是机场节点；相同或代理不通 = 代理未生效。"
