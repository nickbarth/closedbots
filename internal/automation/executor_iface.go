package automation

import (
	"context"

	"github.com/nickbarth/closedbots/internal/domain"
)

type Executor interface {
	Execute(ctx context.Context, action domain.Action) error
	BackendName() string
}
