package ai

import "testing"

func TestLogTextSnippet(t *testing.T) {
	if got := logTextSnippet("  "); got != "" {
		t.Fatalf("got %q", got)
	}
	short := "hello"
	if got := logTextSnippet(short); got != short {
		t.Fatalf("got %q", got)
	}
	long := ""
	for i := 0; i < 5000; i++ {
		long += "x"
	}
	got := logTextSnippet(long)
	if len(got) == 0 || len(got) >= len(long) {
		t.Fatalf("expected clipped output")
	}
}
