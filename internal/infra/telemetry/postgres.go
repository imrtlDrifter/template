package telemetry

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type PostgresTracer struct {
	*otelpgx.Tracer
	duration metric.Float64Histogram
}

type queryKey struct{}

type query struct {
	started    time.Time
	operation  string
	collection string
}

func NewPostgresTracer(provider trace.TracerProvider, duration metric.Float64Histogram) *PostgresTracer {
	return &PostgresTracer{Tracer: otelpgx.NewTracer(otelpgx.WithTracerProvider(provider), otelpgx.WithMeterProvider(noop.NewMeterProvider()), otelpgx.WithDisableSQLStatementInAttributes(), otelpgx.WithDisableConnectionDetailsInAttributes()), duration: duration}
}

func (t *PostgresTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.Tracer.TraceQueryStart(ctx, conn, data)
	operation := otelpgx.SQLOperationName(ctx, data.SQL)
	collection := queryCollection(data.SQL)
	trace.SpanFromContext(ctx).SetAttributes(attribute.String("db.operation.name", operation), attribute.String("db.collection.name", collection))
	return context.WithValue(ctx, queryKey{}, query{started: time.Now(), operation: operation, collection: collection})
}

func (t *PostgresTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if dataQuery, ok := ctx.Value(queryKey{}).(query); ok {
		errorType := ""
		if data.Err != nil {
			errorType = "database_error"
			var pgErr *pgconn.PgError
			if errors.As(data.Err, &pgErr) {
				errorType = pgErr.Code
			}
			trace.SpanFromContext(ctx).SetAttributes(attribute.String("error.type", errorType))
		}
		t.duration.Record(ctx, time.Since(dataQuery.started).Seconds(), metric.WithAttributes(attribute.String("db.operation.name", dataQuery.operation), attribute.String("db.collection.name", dataQuery.collection), attribute.String("error.type", errorType)))
	}
	t.Tracer.TraceQueryEnd(ctx, conn, data)
}

func queryCollection(sql string) string {
	fields := strings.Fields(sql)
	for i, field := range fields {
		switch strings.ToUpper(field) {
		case "INTO", "FROM", "UPDATE":
			if i+1 < len(fields) {
				return strings.Trim(fields[i+1], "\"();")
			}
		}
	}
	return ""
}
