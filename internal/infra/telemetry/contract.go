package telemetry

import "context"

//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

type businessLogger interface {
	InfoContext(ctx context.Context, msg string, args ...any)
}

type tripCounter interface {
	CountActive(ctx context.Context) (int64, error)
}
