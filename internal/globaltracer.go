package internal // import "github.com/pm-esd/tracing/internal"

import (
	"sync"

	"github.com/pm-esd/tracing"
)

var (
	mu           sync.RWMutex   // guards globalTracer
	globalTracer tracing.Tracer = &NoopTracer{}
)

// SetGlobalTracer sets the global tracer to t.
func SetGlobalTracer(t tracing.Tracer) {
	mu.Lock()
	defer mu.Unlock()
	if !Testing {
		// avoid infinite loop when calling (*mocktracer.Tracer).Stop
		globalTracer.Stop()
	}
	globalTracer = t
}

// GetGlobalTracer returns the currently active tracer.
func GetGlobalTracer() tracing.Tracer {
	mu.RLock()
	defer mu.RUnlock()
	return globalTracer
}

// Testing is set to true when the mock tracer is active. It usually signifies that we are in a test
// environment. This value is used by tracer.Start to prevent overriding the GlobalTracer in tests.
var Testing = false

var _ tracing.Tracer = (*NoopTracer)(nil)

// NoopTracer is an implementation of tracing.Tracer that is a no-op.
type NoopTracer struct{}

// StartSpan implements tracing.Tracer.
func (NoopTracer) StartSpan(operationName string, opts ...tracing.StartSpanOption) tracing.Span {
	return NoopSpan{}
}

// SetServiceInfo implements tracing.Tracer.
func (NoopTracer) SetServiceInfo(name, app, appType string) {}

// Extract implements tracing.Tracer.
func (NoopTracer) Extract(carrier interface{}) (tracing.SpanContext, error) {
	return NoopSpanContext{}, nil
}

// Inject implements tracing.Tracer.
func (NoopTracer) Inject(context tracing.SpanContext, carrier interface{}) error { return nil }

// Stop implements tracing.Tracer.
func (NoopTracer) Stop() {}

var _ tracing.Span = (*NoopSpan)(nil)

// NoopSpan is an implementation of tracing.Span that is a no-op.
type NoopSpan struct{}

// SetTag implements tracing.Span.
func (NoopSpan) SetTag(key string, value interface{}) {}

// SetOperationName implements tracing.Span.
func (NoopSpan) SetOperationName(operationName string) {}

// BaggageItem implements tracing.Span.
func (NoopSpan) BaggageItem(key string) string { return "" }

// SetBaggageItem implements tracing.Span.
func (NoopSpan) SetBaggageItem(key, val string) {}

// Finish implements tracing.Span.
func (NoopSpan) Finish(opts ...tracing.FinishOption) {}

// Tracer implements tracing.Span.
func (NoopSpan) Tracer() tracing.Tracer { return NoopTracer{} }

// Context implements tracing.Span.
func (NoopSpan) Context() tracing.SpanContext { return NoopSpanContext{} }

var _ tracing.SpanContext = (*NoopSpanContext)(nil)

// NoopSpanContext is an implementation of tracing.SpanContext that is a no-op.
type NoopSpanContext struct{}

// SpanID implements tracing.SpanContext.
func (NoopSpanContext) SpanID() uint64 { return 0 }

// TraceID implements tracing.SpanContext.
func (NoopSpanContext) TraceID() uint64 { return 0 }

// ForeachBaggageItem implements tracing.SpanContext.
func (NoopSpanContext) ForeachBaggageItem(handler func(k, v string) bool) {}
