package runner

import "testing"

func TestRequiresExplicitVerification(t *testing.T) {
	cases := map[string]bool{
		"":                     false,
		"verify result":        true,
		"make sure it works":   true,
		"check output value":   true,
		"open browser window":  false,
		"navigate to website":  false,
		"type user name":       false,
		"   confirm visually ": false,
	}
	for in, want := range cases {
		if got := requiresExplicitVerification(in); got != want {
			t.Fatalf("in=%q got=%v want=%v", in, got, want)
		}
	}
}

func TestShouldPlanStepActions(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"open browser", true},
		{"verify result", false},
		{"verify and click checkbox", true},
		{"check and go to page", true},
	}
	for _, c := range cases {
		if got := shouldPlanStepActions(c.in); got != c.want {
			t.Fatalf("in=%q got=%v want=%v", c.in, got, c.want)
		}
	}
}
