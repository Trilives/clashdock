package execx

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestSudoArgvRunsInvokedPathWithOriginalArgs(t *testing.T) {
	argv := sudoArgv("/usr/bin/sudo", "/usr/bin/clashdock", []string{"update"}, envFrom(nil))

	want := []string{"/usr/bin/sudo", "--", "/usr/bin/clashdock", "update"}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %q, want %q (nothing to preserve, so restrictive sudoers still work)", argv, want)
	}
}

func TestSudoArgvPreservesSetVariablesByNameOnly(t *testing.T) {
	env := envFrom(map[string]string{"CLASHDOCK_LANG": "zh", "GITHUB_TOKEN": "ghp_secret", "HOME": "/home/u"})

	argv := sudoArgv("/usr/bin/sudo", "/usr/bin/clashdock", nil, env)

	want := []string{"/usr/bin/sudo", "--preserve-env=CLASHDOCK_LANG,GITHUB_TOKEN", "--", "/usr/bin/clashdock"}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	if strings.Contains(strings.Join(argv, " "), "ghp_secret") {
		t.Fatal("token value must never appear on the sudo command line (visible to every local user via ps)")
	}
}

func TestSudoArgvNeverCarriesStateDirOverrideIntoRoot(t *testing.T) {
	env := envFrom(map[string]string{"CLASHDOCK_HOME": "/home/u/planted"})

	argv := sudoArgv("/usr/bin/sudo", "/usr/bin/clashdock", nil, env)

	if strings.Contains(strings.Join(argv, " "), "CLASHDOCK_HOME") {
		t.Fatalf("argv = %q: root must not take over / execute from a directory the unprivileged environment picked", argv)
	}
}

func TestReexecAsRootStopsAfterOneSudoHop(t *testing.T) {
	if IsRoot() {
		t.Skip("running as root")
	}
	t.Setenv("SUDO_UID", "1000")

	err := ReexecAsRoot("/usr/bin/clashdock", nil)

	if err == nil || !strings.Contains(err.Error(), "sudo") {
		t.Fatalf("ReexecAsRoot after a sudo hop = %v, want an explanatory error instead of looping", err)
	}
}

func TestStableExecutablePathKeepsInvokedPath(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "clashdock" {
			return "/usr/bin/clashdock", nil
		}
		return "", errors.New("not found")
	}
	tests := []struct {
		arg0 string
		want string
	}{
		{"/usr/bin/clashdock", "/usr/bin/clashdock"}, // 符号链接本身，不解析到版本目录
		{"clashdock", "/usr/bin/clashdock"},          // 经 PATH 查找
	}
	for _, tt := range tests {
		got, err := stableExecutablePath(tt.arg0, lookPath)
		if err != nil || got != tt.want {
			t.Errorf("stableExecutablePath(%q) = %q, %v; want %q", tt.arg0, got, err, tt.want)
		}
	}
	if _, err := stableExecutablePath("missing", lookPath); err == nil {
		t.Error("an unresolvable bare name must be an error")
	}
}

func TestStableExecutablePathMakesRelativePathAbsolute(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	got, err := stableExecutablePath("./clashdock", nil)

	if err != nil || got != wd+"/clashdock" {
		t.Fatalf("stableExecutablePath(./clashdock) = %q, %v; want %q", got, err, wd+"/clashdock")
	}
}

func TestInvokingUserHomeIgnoresSudoUserWhenNotRoot(t *testing.T) {
	if IsRoot() {
		t.Skip("running as root")
	}
	t.Setenv("SUDO_USER", "root-should-not-matter")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}

	if got := InvokingUserHome(); got != home {
		t.Fatalf("InvokingUserHome = %q, want own home %q when not running as root", got, home)
	}
}
