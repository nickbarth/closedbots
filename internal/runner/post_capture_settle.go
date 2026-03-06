package runner

import "github.com/nickbarth/closedbots/internal/domain"

const postActionSettleMs = 250

func shouldSettleBeforePostCapture(actions []domain.Action) bool {
	if len(actions) == 0 {
		return false
	}
	last := actions[len(actions)-1]
	return last.Type != domain.ActionWait
}
