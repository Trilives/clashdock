package usermode

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// capData 构造 vfs_cap_data v2：magic_etc + 两组 permitted/inheritable。
func capData(magic, permitted uint32) []byte {
	b := make([]byte, 20)
	binary.LittleEndian.PutUint32(b[0:4], magic)
	binary.LittleEndian.PutUint32(b[4:8], permitted)
	return b
}

func TestParseTunCaps(t *testing.T) {
	all := uint32(1<<capNetAdmin | 1<<capNetRaw | 1<<capNetBindService)
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"all caps effective v2", capData(0x02000000|vfsCapFlagsEff, all), true},
		{"all caps effective v3", append(capData(0x03000000|vfsCapFlagsEff, all), 0, 0, 0, 0), true},
		{"missing effective flag", capData(0x02000000, all), false},
		{"missing net_raw", capData(0x02000000|vfsCapFlagsEff, 1<<capNetAdmin|1<<capNetBindService), false},
		{"unknown revision", capData(0x09000000|vfsCapFlagsEff, all), false},
		{"truncated", []byte{1, 2, 3}, false},
	}
	for _, c := range cases {
		if got := parseTunCaps(c.data); got != c.want {
			t.Errorf("%s: parseTunCaps = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestHasTunCapsFalseForPlainFile(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "mihomo")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if HasTunCaps(bin) {
		t.Fatal("a file without security.capability must not report TUN caps")
	}
	if HasTunCaps(filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("a missing file must not report TUN caps")
	}
}

func TestWaitTunDevice(t *testing.T) {
	dir := t.TempDir()
	old := sysClassNet
	sysClassNet = dir
	t.Cleanup(func() { sysClassNet = old })

	if WaitTunDevice("mihomo", 0) {
		t.Fatal("absent device must report false")
	}
	if err := os.Mkdir(filepath.Join(dir, "mihomo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !WaitTunDevice("mihomo", time.Second) {
		t.Fatal("present device must report true")
	}
}
