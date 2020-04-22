package tracing

import (
	"context"
	"io"
	"net/http"
	"runtime"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-lib/metrics/prometheus"
)

// LogrusAdapter - an adapter to log span info
type LogrusAdapter struct {
	// InfoLevel bool
}

// Error - logrus adapter for span errors
func (l LogrusAdapter) Error(msg string) {
	logrus.Error(msg)
}

// Infof - logrus adapter for span info logging
func (l LogrusAdapter) Infof(msg string, args ...interface{}) {
	logrus.Infof(msg, args...)
}

// Errorf - logrus adapter for span info logging
func (l LogrusAdapter) Errorf(msg string, args ...interface{}) {
	logrus.Errorf(msg, args...)
}

// Option - define options for NewJWTCache()
type Option func(*options)
type options struct {
	sampleProbability float64
	enableInfoLog     bool
}

// defaultOptions - some defs options to NewJWTCache()
var defaultOptions = options{
	sampleProbability: 0.0,
	enableInfoLog:     false,
}

// WithSampleProbability - optional sample probability
func WithSampleProbability(sampleProbability float64) Option {
	return func(o *options) {
		o.sampleProbability = sampleProbability
	}
}

// WithEnableInfoLog - optional: enable Info logging for tracing
func WithEnableInfoLog(enable bool) Option {
	return func(o *options) {
		o.enableInfoLog = enable
	}
}

// InitTracing - init opentracing with options (WithSampleProbability, WithEnableInfoLog) defaults: constant sampling, no info logging
func InitTracing(serviceName string, tracingAgentHostPort string, opt ...Option) (
	tracer opentracing.Tracer,
	reporter jaeger.Reporter,
	closer io.Closer,
	err error) {
	opts := defaultOptions
	for _, o := range opt {
		o(&opts)
	}
	factory := prometheus.New()
	metrics := jaeger.NewMetrics(factory, map[string]string{"lib": "jaeger"})
	transport, err := jaeger.NewUDPTransport(tracingAgentHostPort, 0)
	if err != nil {
		return tracer, reporter, closer, err
	}

	logAdapt := LogrusAdapter{}
	reporter = jaeger.NewCompositeReporter(
		jaeger.NewLoggingReporter(logAdapt),
		jaeger.NewRemoteReporter(transport,
			jaeger.ReporterOptions.Metrics(metrics),
			jaeger.ReporterOptions.Logger(logAdapt),
		),
	)
	if opts.sampleProbability > 0 {
		sampler, _ := jaeger.NewProbabilisticSampler(opts.sampleProbability)
		tracer, closer = jaeger.NewTracer(serviceName,
			sampler,
			reporter,
			jaeger.TracerOptions.Metrics(metrics),
		)
		return tracer, reporter, closer, nil
	} else {
		sampler := jaeger.NewConstSampler(true)
		tracer, closer = jaeger.NewTracer(serviceName,
			sampler,
			reporter,
			jaeger.TracerOptions.Metrics(metrics),
		)
		return tracer, reporter, closer, nil
	}
}

// HTTPSpan starts a new HTTP span.
func HTTPSpan(path string, r *http.Request) (opentracing.Span, *http.Request) {
	ctx, err := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(r.Header),
	)
	if err != nil && err != opentracing.ErrSpanContextNotFound {
		logrus.Errorf("failed to extract HTTP span: %w", err)
	}
	sp := opentracing.StartSpan(
		HTTPOpName(r.Method, path),
		ext.RPCServerOption(ctx),
	)
	ext.HTTPMethod.Set(sp, r.Method)
	ext.HTTPUrl.Set(sp, r.URL.String())
	ext.Component.Set(sp, "http")
	sp.SetTag("goroutines", runtime.NumGoroutine())
	return sp, r.WithContext(
		opentracing.ContextWithSpan(
			r.Context(),
			sp,
		),
	)
}

// HTTPOpName return a string representation of the HTTP request operation.
func HTTPOpName(method, path string) string {
	return method + " " + path
}

// FinishHTTPSpan finishes a HTTP span by providing a HTTP status code.
func FinishHTTPSpan(sp opentracing.Span, code int) {
	ext.HTTPStatusCode.Set(sp, uint16(code))
	sp.Finish()
}

// ConsumerSpan starts a new consumer span.
func ConsumerSpan(ctx context.Context, opName, cmp string, hdr map[string]string, tags ...opentracing.Tag) (opentracing.Span, context.Context) {
	spCtx, err := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.TextMapCarrier(hdr),
	)
	if err != nil && err != opentracing.ErrSpanContextNotFound {
		logrus.Errorf("failed to extract consumer span: %v", err)
	}
	sp := opentracing.StartSpan(
		opName,
		consumerOption{ctx: spCtx},
	)
	ext.Component.Set(sp, cmp)
	sp.SetTag("goroutines", runtime.NumGoroutine())
	for _, t := range tags {
		sp.SetTag(t.Key, t.Value)
	}
	return sp, opentracing.ContextWithSpan(ctx, sp)
}

// SpanSuccess finishes a span with a success indicator.
func SpanSuccess(sp opentracing.Span) {
	ext.Error.Set(sp, false)
	sp.Finish()
}

// SpanError finishes a span with a error indicator.
func SpanError(sp opentracing.Span) {
	ext.Error.Set(sp, true)
	sp.Finish()
}

// ChildSpan starts a new child span with specified tags.
func ChildSpan(ctx context.Context, opName, cmp string, tags ...opentracing.Tag) (opentracing.Span, context.Context) {
	sp, ctx := opentracing.StartSpanFromContext(ctx, opName)
	ext.Component.Set(sp, cmp)
	for _, t := range tags {
		sp.SetTag(t.Key, t.Value)
	}
	sp.SetTag("goroutines", runtime.NumGoroutine())
	return sp, ctx
}

// SQLSpan starts a new SQL child span with specified tags.
func SQLSpan(ctx context.Context, opName, cmp, sqlType, instance, user, stmt string, tags ...opentracing.Tag) (opentracing.Span, context.Context) {
	sp, ctx := opentracing.StartSpanFromContext(ctx, opName)
	ext.Component.Set(sp, cmp)
	ext.DBType.Set(sp, sqlType)
	ext.DBInstance.Set(sp, instance)
	ext.DBUser.Set(sp, user)
	ext.DBStatement.Set(sp, stmt)
	for _, t := range tags {
		sp.SetTag(t.Key, t.Value)
	}
	sp.SetTag("goroutines", runtime.NumGoroutine())
	return sp, ctx
}

type consumerOption struct {
	ctx opentracing.SpanContext
}

func (r consumerOption) Apply(o *opentracing.StartSpanOptions) {
	if r.ctx != nil {
		opentracing.ChildOf(r.ctx).Apply(o)
	}
	ext.SpanKindConsumer.Apply(o)
}

// ComponentOpName returns a operation name for a component.
func ComponentOpName(cmp, target string) string {
	return cmp + " " + target
}
