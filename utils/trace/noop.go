package trace

import "go.opentelemetry.io/otel/trace/noop"

var Noop Tracer = noOpTracer{}

// noOpTracer is an implementation of trace.Tracer that does nothing.
type noOpTracer struct {
	noop.Tracer
}

func (noOpTracer) Close() error {
	return nil
}
