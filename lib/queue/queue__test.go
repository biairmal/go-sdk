package queue

import (
	"context"
	"testing"

	"github.com/biairmal/go-sdk/lib/logger"
)

// fakeLogger embeds logger.Logger (left nil) and overrides only Info, the
// one method NewLogging's Publish calls — the untouched embedded methods
// are never exercised by this test.
type fakeLogger struct {
	logger.Logger
	infoCalls int
}

func (f *fakeLogger) Info(_ string, _ ...logger.Field) {
	f.infoCalls++
}

func TestNewNoOp_DiscardsAndReturnsNil(t *testing.T) {
	pub := NewNoOp()
	if err := pub.Publish(context.Background(), "topic", []byte("payload")); err != nil {
		t.Fatalf("Publish() error = %v, want nil", err)
	}
}

func TestNewLogging_LogsAndReturnsNil(t *testing.T) {
	log := &fakeLogger{}
	pub := NewLogging(log)
	if err := pub.Publish(context.Background(), "topic", []byte("payload")); err != nil {
		t.Fatalf("Publish() error = %v, want nil", err)
	}
	if log.infoCalls != 1 {
		t.Fatalf("Info() calls = %d, want 1", log.infoCalls)
	}
}

func TestApplyOptions(t *testing.T) {
	o := applyOptions([]PublishOption{
		WithKey([]byte("k")),
		WithHeaders(map[string]string{"h": "v"}),
	})
	if string(o.key) != "k" {
		t.Errorf("key = %q, want %q", o.key, "k")
	}
	if o.headers["h"] != "v" {
		t.Errorf("headers[h] = %q, want %q", o.headers["h"], "v")
	}
}

func TestApplyOptions_Empty(t *testing.T) {
	o := applyOptions(nil)
	if o.key != nil || o.headers != nil {
		t.Errorf("applyOptions(nil) = %+v, want zero value", o)
	}
}
