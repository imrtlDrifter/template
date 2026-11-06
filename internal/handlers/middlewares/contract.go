package middlewares

import "context"

type errorLogger interface {
	ErrorContext(ctx context.Context, err error, msg string, extra ...any)
}
