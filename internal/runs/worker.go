package runs

import (
	"context"
	"log/slog"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/internal/storage"
)

type Worker struct {
	Store   *storage.Store
	Plans   c.PlanService
	Timeout time.Duration
}

func (w *Worker) Serve(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		id, cmd, e := w.Store.Claim(ctx)
		if e != nil {
			slog.Error("claim run", "error", e)
		} else if id != "" {
			w.execute(ctx, id, cmd)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (w *Worker) execute(parent context.Context, id string, cmd storage.Command) {
	ctx, cancel := context.WithTimeout(parent, w.Timeout)
	defer cancel()
	var result c.PlanResult
	var e error
	var in c.PlanCommit
	if cmd.Kind == "build" {
		result, e = w.Plans.Build(ctx, *cmd.Build)
		in = c.PlanCommit{RequestID: cmd.Build.RequestID, ExpectedRevision: cmd.Build.SnapshotRevision, ExpectedCurrentPlanID: cmd.Build.ExpectedCurrentPlanID}
	} else {
		result, e = w.Plans.Replan(ctx, *cmd.Replan)
		in = c.PlanCommit{RequestID: cmd.Replan.RequestID, ExpectedRevision: cmd.Replan.SnapshotRevision, ExpectedCurrentPlanID: &cmd.Replan.BasePlanID}
	}
	if e == nil {
		in.Result = result
		_, e = w.Store.CommitPlan(ctx, in)
	}
	if e != nil {
		slog.Error("run failed", "run_id", id, "error", e)
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.Store.Fail(saveCtx, id, e); err != nil {
			slog.Error("save failed run", "run_id", id, "error", err)
		}
	}
}
