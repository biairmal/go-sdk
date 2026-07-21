package ctxkit

import (
	"context"
	"testing"
)

func TestContextValueRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		with func(context.Context, string) context.Context
		get  func(context.Context) string
	}{
		{name: "request id", with: WithRequestID, get: RequestID},
		{name: "correlation id", with: WithCorrelationID, get: CorrelationID},
		{name: "trace id", with: WithTraceID, get: TraceID},
		{name: "user id", with: WithUserID, get: UserID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.with(context.Background(), "value-123")
			if got := tt.get(ctx); got != "value-123" {
				t.Errorf("get after set = %q, want %q", got, "value-123")
			}
		})
	}
}

func TestGettersReturnEmptyWhenAbsent(t *testing.T) {
	ctx := context.Background()
	getters := map[string]func(context.Context) string{
		"request id":     RequestID,
		"correlation id": CorrelationID,
		"trace id":       TraceID,
		"user id":        UserID,
	}
	for name, get := range getters {
		t.Run(name, func(t *testing.T) {
			if got := get(ctx); got != "" {
				t.Errorf("get on empty context = %q, want empty", got)
			}
		})
	}
}

func TestSettersIgnoreEmptyValues(t *testing.T) {
	ctx := WithRequestID(context.Background(), "")
	if ctx != context.Background() {
		t.Error("WithRequestID with empty value should return the original context unchanged")
	}
	if got := RequestID(ctx); got != "" {
		t.Errorf("RequestID after empty set = %q, want empty", got)
	}
}

func TestSettersAreIndependent(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req")
	ctx = WithCorrelationID(ctx, "corr")
	ctx = WithTraceID(ctx, "trace")
	ctx = WithUserID(ctx, "user")

	if RequestID(ctx) != "req" || CorrelationID(ctx) != "corr" ||
		TraceID(ctx) != "trace" || UserID(ctx) != "user" {
		t.Errorf("independent values collided: req=%q corr=%q trace=%q user=%q",
			RequestID(ctx), CorrelationID(ctx), TraceID(ctx), UserID(ctx))
	}
}

func TestLoggerExtractorEmitsPresentFields(t *testing.T) {
	tests := []struct {
		name   string
		ctx    func() context.Context
		want   map[string]string
		length int
	}{
		{
			name:   "empty context yields no fields",
			ctx:    context.Background,
			want:   map[string]string{},
			length: 0,
		},
		{
			name: "only request id",
			ctx: func() context.Context {
				return WithRequestID(context.Background(), "req-1")
			},
			want:   map[string]string{"request_id": "req-1"},
			length: 1,
		},
		{
			name: "all four fields",
			ctx: func() context.Context {
				ctx := WithRequestID(context.Background(), "req-1")
				ctx = WithCorrelationID(ctx, "corr-1")
				ctx = WithTraceID(ctx, "trace-1")
				ctx = WithUserID(ctx, "user-1")
				return ctx
			},
			want: map[string]string{
				"request_id":     "req-1",
				"correlation_id": "corr-1",
				"trace_id":       "trace-1",
				"user_id":        "user-1",
			},
			length: 4,
		},
	}

	extract := LoggerExtractor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := extract(tt.ctx())
			if len(fields) != tt.length {
				t.Fatalf("field count = %d, want %d", len(fields), tt.length)
			}
			for _, f := range fields {
				want, ok := tt.want[f.Key]
				if !ok {
					t.Errorf("unexpected field %q", f.Key)
					continue
				}
				if f.Value != want {
					t.Errorf("field %q = %v, want %q", f.Key, f.Value, want)
				}
			}
		})
	}
}
