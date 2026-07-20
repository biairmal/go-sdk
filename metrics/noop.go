package metrics

// NewNoOp returns a Recorder that discards everything. Use it in tests and
// in environments where metrics are disabled, without branching call sites
// on a nil Recorder.
func NewNoOp() Recorder {
	return noOpRecorder{}
}

// noOpRecorder implements Recorder with no-op methods.
type noOpRecorder struct{}

func (noOpRecorder) CounterInc(_ string, _ Labels)                  {}
func (noOpRecorder) HistogramObserve(_ string, _ float64, _ Labels) {}
func (noOpRecorder) GaugeAdd(_ string, _ float64, _ Labels)         {}
