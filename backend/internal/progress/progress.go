package progress

import "context"

// Update describes completed route variants and the current calculation phase.
// Completed/Total count variants, not elapsed time or an estimated percentage.
type Update struct {
	Stage        string `json:"stage"`
	Message      string `json:"message"`
	Completed    int    `json:"completed"`
	Total        int    `json:"total"`
	VariantLabel string `json:"variant_label,omitempty"`
}

type reporterKey struct{}
type stateKey struct{}

type State struct {
	Completed int
	Total     int
	Label     string
}

func WithReporter(ctx context.Context, report func(Update)) context.Context {
	return context.WithValue(ctx, reporterKey{}, report)
}

func WithState(ctx context.Context, state State) context.Context {
	return context.WithValue(ctx, stateKey{}, state)
}

func Report(ctx context.Context, stage, message string) {
	report, ok := ctx.Value(reporterKey{}).(func(Update))
	if !ok || report == nil {
		return
	}
	state, _ := ctx.Value(stateKey{}).(State)
	report(Update{Stage: stage, Message: message, Completed: state.Completed, Total: state.Total, VariantLabel: state.Label})
}
