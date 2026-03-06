package runner

import (
	"testing"

	"github.com/nickbarth/closedbots/internal/domain"
)

func TestShouldSettleBeforePostCapture(t *testing.T) {
	if shouldSettleBeforePostCapture(nil) {
		t.Fatalf("nil actions should be false")
	}
	if shouldSettleBeforePostCapture([]domain.Action{{Type: domain.ActionWait}}) {
		t.Fatalf("wait tail should be false")
	}
	if !shouldSettleBeforePostCapture([]domain.Action{{Type: domain.ActionClick}}) {
		t.Fatalf("non-wait tail should be true")
	}
}
