package tui

import (
	"io"
	"os"
	"testing"
)

// 非 TTY 回退按行读取时不得预读 stdin：交接给 exec 的新进程或 sudo 等子进程共享同一
// stdin，被预读进缓冲的后续答案会凭空丢失。
func TestReadLineDoesNotConsumeBeyondNewline(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := w.WriteString("first\nsecond\nthird\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()

	line, err := readLine(r)
	if err != nil || line != "first\n" {
		t.Fatalf("readLine = %q, %v", line, err)
	}
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(rest) != "second\nthird\n" {
		t.Fatalf("remaining stdin = %q; lines after the first must stay unread", rest)
	}
}

func TestReadLineReturnsPartialLineAtEOF(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	w.WriteString("tail")
	w.Close()
	line, err := readLine(r)
	if line != "tail" || err != io.EOF {
		t.Fatalf("readLine = %q, %v; want partial line with EOF", line, err)
	}
}
