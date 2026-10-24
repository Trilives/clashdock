package proxyenv

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestBlockUsesConfiguredPort(t *testing.T) {
	got := block(7899)
	for _, want := range []string{beginMark, endMark, "http://127.0.0.1:7899", "socks5://127.0.0.1:7899"} {
		if !strings.Contains(got, want) {
			t.Errorf("block missing %q:\n%s", want, got)
		}
	}
}

func TestStripBlockRemovesOnlyManagedBlock(t *testing.T) {
	text := "alias ll='ls -l'\n" + block(7890) + "\nexport EDITOR=vim\n"

	got := stripBlock(text)

	if strings.Contains(got, beginMark) || strings.Contains(got, "http_proxy") {
		t.Fatalf("managed block left behind:\n%s", got)
	}
	for _, keep := range []string{"alias ll='ls -l'", "export EDITOR=vim"} {
		if !strings.Contains(got, keep) {
			t.Errorf("user line %q removed:\n%s", keep, got)
		}
	}
}

// rewrite 一个总是改写为 data 的 edit，并记下它看到的旧内容。
func rewrite(data string, seen *string) func(string) (string, bool) {
	return func(old string) (string, bool) {
		*seen = old
		return data, true
	}
}

func TestEditOwnedFileRefusesSymlinkWithoutReadingTarget(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "id_ed25519")
	if err := os.WriteFile(victim, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	bashrc := filepath.Join(dir, ".bashrc")
	if err := os.Symlink(victim, bashrc); err != nil {
		t.Fatal(err)
	}
	var seen string

	err := editOwnedFile(bashrc, os.Getuid(), os.Getgid(), rewrite("export x=1\n", &seen))

	if err == nil {
		t.Fatal("a symlinked ~/.bashrc must be refused: root would read and overwrite its target")
	}
	if seen != "" {
		t.Fatalf("root read the symlink target into the edit: %q", seen)
	}
	if got, _ := os.ReadFile(victim); string(got) != "PRIVATE KEY" {
		t.Fatalf("symlink target modified: %q", got)
	}
}

func TestEditOwnedFileRefusesFifoWithoutBlocking(t *testing.T) {
	bashrc := filepath.Join(t.TempDir(), ".bashrc")
	if err := syscall.Mkfifo(bashrc, 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan error, 1)
	var seen string

	go func() { done <- editOwnedFile(bashrc, os.Getuid(), os.Getgid(), rewrite("x", &seen)) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a FIFO at ~/.bashrc must be refused")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("editOwnedFile blocked on a FIFO: root would hang")
	}
}

func TestEditOwnedFileRefusesFileOfAnotherUser(t *testing.T) {
	bashrc := filepath.Join(t.TempDir(), ".bashrc")
	if err := os.WriteFile(bashrc, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	var seen string

	if err := editOwnedFile(bashrc, os.Getuid()+1, os.Getgid(), rewrite("x", &seen)); err == nil {
		t.Fatal("a file not owned by the target user must be refused")
	}
	if seen != "" {
		t.Fatalf("content of a foreign file reached the edit: %q", seen)
	}
}

func TestEditOwnedFileRefusesHardLinkedFile(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "other")
	if err := os.WriteFile(other, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	bashrc := filepath.Join(dir, ".bashrc")
	if err := os.Link(other, bashrc); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	var seen string

	if err := editOwnedFile(bashrc, os.Getuid(), os.Getgid(), rewrite("x", &seen)); err == nil {
		t.Fatal("a hard-linked ~/.bashrc must be refused")
	}
	if got, _ := os.ReadFile(other); string(got) != "keep" {
		t.Fatalf("hard link target modified: %q", got)
	}
}

func TestEditOwnedFileCreatesThenEditsOwnFile(t *testing.T) {
	bashrc := filepath.Join(t.TempDir(), ".bashrc")
	var seen string

	if err := editOwnedFile(bashrc, os.Getuid(), os.Getgid(), rewrite("first line that is longer\n", &seen)); err != nil {
		t.Fatal(err)
	}
	if err := editOwnedFile(bashrc, os.Getuid(), os.Getgid(), rewrite("second\n", &seen)); err != nil {
		t.Fatal(err)
	}

	if seen != "first line that is longer\n" {
		t.Fatalf("edit saw %q, want the existing content read through the checked handle", seen)
	}
	if got, _ := os.ReadFile(bashrc); string(got) != "second\n" {
		t.Fatalf("content = %q, want the rewrite to replace (not append to) the old content", got)
	}
}

func TestEditOwnedFileLeavesMissingFileAloneWhenNothingToWrite(t *testing.T) {
	bashrc := filepath.Join(t.TempDir(), ".bashrc")

	err := editOwnedFile(bashrc, os.Getuid(), os.Getgid(), func(string) (string, bool) { return "", false })

	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(bashrc); !os.IsNotExist(err) {
		t.Fatalf("Remove on a missing bashrc must not create it, lstat err = %v", err)
	}
}
