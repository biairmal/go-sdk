package ctxkit

import (
	"context"

	"github.com/biairmal/go-sdk/lib/logger"
)

// LoggerExtractor returns a logger.ContextExtractor that emits every canonical
// field present in the context (request_id, correlation_id, trace_id, user_id).
// Absent fields are omitted.
//
// Wire it once at logger construction so every *WithContext log call carries the
// values automatically:
//
//	log := logger.NewZerolog(&logger.Options{
//		ContextExtractor: ctxkit.LoggerExtractor(),
//	})
func LoggerExtractor() logger.ContextExtractor {
	return func(ctx context.Context) []logger.Field {
		fields := make([]logger.Field, 0, 4)
		if v := RequestID(ctx); v != "" {
			fields = append(fields, logger.Field{Key: "request_id", Value: v})
		}
		if v := CorrelationID(ctx); v != "" {
			fields = append(fields, logger.Field{Key: "correlation_id", Value: v})
		}
		if v := TraceID(ctx); v != "" {
			fields = append(fields, logger.Field{Key: "trace_id", Value: v})
		}
		if v := UserID(ctx); v != "" {
			fields = append(fields, logger.Field{Key: "user_id", Value: v})
		}
		return fields
	}
}
