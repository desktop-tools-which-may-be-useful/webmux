package workspace

import (
	"context"
	"errors"

	"github.com/rainchan/webmux/internal/session"
)

// CreateSession creates a session through Create and returns it, with the
// failed step's own error: the single-step form most tests need.
func (s *Service) CreateSession(ctx context.Context, in CreateInput) (*session.Session, error) {
	result, err := s.Create(ctx, in)
	var failed *CreateError
	if errors.As(err, &failed) {
		return nil, failed.Err
	}
	return result.Session, err
}
