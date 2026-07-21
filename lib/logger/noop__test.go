package logger

import (
	"context"
	"testing"
)

type testContextKey string

func TestNewNoOp(t *testing.T) {
	log := NewNoOp()
	if log == nil {
		t.Fatal("NewNoOp() returned nil")
	}

	// Verify it implements Logger interface
	var _ Logger = log
}

func TestNoOpLogger_AllMethods(t *testing.T) {
	log := NewNoOp()
	ctx := context.WithValue(context.Background(), testContextKey("test"), "value")

	tests := []struct {
		name string
		fn   func()
	}{
		{
			name: "Debug",
			fn:   func() { log.Debug("test", F("key", "value")) },
		},
		{
			name: "Info",
			fn:   func() { log.Info("test", F("key", "value")) },
		},
		{
			name: "Warn",
			fn:   func() { log.Warn("test", F("key", "value")) },
		},
		{
			name: "Error",
			fn:   func() { log.Error("test", F("key", "value")) },
		},
		{
			name: "Fatal",
			fn:   func() { log.Fatal("test", F("key", "value")) },
		},
		{
			name: "Panic",
			fn:   func() { log.Panic("test", F("key", "value")) },
		},
		{
			name: "Debugf",
			fn:   func() { log.Debugf("test %s", "value") },
		},
		{
			name: "Infof",
			fn:   func() { log.Infof("test %s", "value") },
		},
		{
			name: "Warnf",
			fn:   func() { log.Warnf("test %s", "value") },
		},
		{
			name: "Errorf",
			fn:   func() { log.Errorf("test %s", "value") },
		},
		{
			name: "Fatalf",
			fn:   func() { log.Fatalf("test %s", "value") },
		},
		{
			name: "Panicf",
			fn:   func() { log.Panicf("test %s", "value") },
		},
		{
			name: "DebugWithContext",
			fn:   func() { log.DebugWithContext(ctx, "test", F("key", "value")) },
		},
		{
			name: "InfoWithContext",
			fn:   func() { log.InfoWithContext(ctx, "test", F("key", "value")) },
		},
		{
			name: "WarnWithContext",
			fn:   func() { log.WarnWithContext(ctx, "test", F("key", "value")) },
		},
		{
			name: "ErrorWithContext",
			fn:   func() { log.ErrorWithContext(ctx, "test", F("key", "value")) },
		},
		{
			name: "FatalWithContext",
			fn:   func() { log.FatalWithContext(ctx, "test", F("key", "value")) },
		},
		{
			name: "PanicWithContext",
			fn:   func() { log.PanicWithContext(ctx, "test", F("key", "value")) },
		},
		{
			name: "DebugfWithContext",
			fn:   func() { log.DebugfWithContext(ctx, "test %s", "value") },
		},
		{
			name: "InfofWithContext",
			fn:   func() { log.InfofWithContext(ctx, "test %s", "value") },
		},
		{
			name: "WarnfWithContext",
			fn:   func() { log.WarnfWithContext(ctx, "test %s", "value") },
		},
		{
			name: "ErrorfWithContext",
			fn:   func() { log.ErrorfWithContext(ctx, "test %s", "value") },
		},
		{
			name: "FatalfWithContext",
			fn:   func() { log.FatalfWithContext(ctx, "test %s", "value") },
		},
		{
			name: "PanicfWithContext",
			fn:   func() { log.PanicfWithContext(ctx, "test %s", "value") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// All no-op methods should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s panicked: %v", tt.name, r)
				}
			}()
			tt.fn()
		})
	}
}
