package fetchx

import "testing"

func TestNewOrderedTriesCandidatesThenDirect(t *testing.T) {
	f := NewOrdered([]string{"http://proxy-a:1", "", "http://proxy-b:2"}, "")
	got := f.attempts()
	want := []string{"http://proxy-a:1", "http://proxy-b:2", ""}
	if len(got) != len(want) {
		t.Fatalf("attempts() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempts()[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestNewOrderedAllEmptyIsDirectOnly(t *testing.T) {
	f := NewOrdered(nil, "")
	got := f.attempts()
	if len(got) != 1 || got[0] != "" {
		t.Fatalf("attempts() = %v, want [\"\"] (direct only)", got)
	}
}
