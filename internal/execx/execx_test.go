package execx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteRootCommandReadsContentFromStdin(t *testing.T) {
	cmd := writeRootCmd("/etc/systemd/system/x.service", "0644")

	for _, arg := range cmd {
		if strings.HasPrefix(arg, os.TempDir()) {
			t.Fatalf("cmd %q installs from a caller-owned temp file that another process of the user could swap before root reads it", cmd)
		}
	}
	if cmd[len(cmd)-2] != "/dev/stdin" {
		t.Fatalf("cmd %q, want content streamed via /dev/stdin", cmd)
	}
}

func TestRunFeedsStdinToInstall(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "unit")

	_, err := Run(writeRootCmd(dst, "0640"), &Opt{Capture: true, Stdin: strings.NewReader("ExecStart=/usr/bin/clashdock\n")})

	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "ExecStart=/usr/bin/clashdock\n" {
		t.Fatalf("installed content = %q, %v", got, err)
	}
	if info, _ := os.Stat(dst); info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v, want 0640", info.Mode().Perm())
	}
}
