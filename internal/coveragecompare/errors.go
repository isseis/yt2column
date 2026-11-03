package coveragecompare

// StopError is a reason to stop without a verdict. Kind is never matched
// from the text of Reasons.
type StopError struct {
	Kind    StopKind
	Reasons []StopReason
	Err     error // the underlying failure, for StopInternal
}

// Error returns the first reason's detail, or the underlying error.
func (e *StopError) Error() string {
	if e == nil {
		return "coveragecompare: nil stop error"
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	if len(e.Reasons) > 0 && e.Reasons[0].Detail != "" {
		return e.Reasons[0].Detail
	}
	return "coveragecompare stopped: " + e.Kind.String()
}

// Unwrap returns the underlying failure.
func (e *StopError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// stopError builds an unreadable StopError with one reason. line is the input
// line number, or 0 when the reason applies to no single line.
func stopError(point Point, input Input, line int, detail, excerpt string) *StopError {
	return &StopError{
		Kind: StopUnreadable,
		Reasons: []StopReason{{
			Point:   point,
			Input:   input,
			Line:    line,
			Detail:  detail,
			Excerpt: excerpt,
		}},
	}
}
