package httpapi

import (
	"context"
	"net/http"
	"strings"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
)

// compare calculates both algorithms from the snapshot used by the first plan.
// It leaves the dispatcher's current plan and event history untouched.
func (s *Server) compare(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ExpectedCurrentPlanID string `json:"expected_current_plan_id"`
	}
	if err := decode(r, &in); err != nil {
		failure(w, err)
		return
	}
	if strings.TrimSpace(in.ExpectedCurrentPlanID) == "" {
		failure(w, invalid("Требуется expected_current_plan_id"))
		return
	}
	scenarioID := r.PathValue("id")
	current, err := s.Store.GetScenario(r.Context(), scenarioID, 0)
	if err != nil {
		failure(w, err)
		return
	}
	if current.CurrentPlanID == nil || *current.CurrentPlanID != in.ExpectedCurrentPlanID {
		failure(w, c.NewError("STALE_VERSION", "Текущий план изменился. Обновите смену."))
		return
	}
	planID := *current.CurrentPlanID
	seen := map[string]bool{}
	var root c.Plan
	for {
		if seen[planID] {
			failure(w, c.NewError("INVALID_PLAN", "Цикл в истории планов"))
			return
		}
		seen[planID] = true
		root, err = s.Store.GetPlan(r.Context(), planID)
		if err != nil {
			failure(w, err)
			return
		}
		if root.ScenarioID != scenarioID {
			failure(w, c.NewError("INVALID_PLAN", "План другого сценария"))
			return
		}
		if root.BasePlanID == nil {
			break
		}
		planID = *root.BasePlanID
	}
	source, err := s.Store.GetScenario(r.Context(), scenarioID, root.SnapshotRevision)
	if err != nil {
		failure(w, err)
		return
	}
	respondCalculation(w, r, http.StatusOK, 2, func(ctx context.Context) (any, error) {
		ctxBaseline := progress.WithState(ctx, progress.State{Completed: 0, Total: 2, Label: "Базовый алгоритм"})
		progress.Report(ctxBaseline, "variant", "Рассчитываем базовый алгоритм")
		baseline, err := s.Plans.Build(ctxBaseline, c.BuildPlanRequest{
			RequestID: "comparison-baseline", ScenarioID: scenarioID,
			SnapshotRevision: root.SnapshotRevision, SolveMode: c.SolveModeBaseline,
		})
		if err != nil {
			return nil, err
		}
		progress.Report(progress.WithState(ctx, progress.State{Completed: 1, Total: 2, Label: "Базовый алгоритм"}), "variant_complete", "Базовый алгоритм готов")
		ctxOptimized := progress.WithState(ctx, progress.State{Completed: 1, Total: 2, Label: "Оптимизированный алгоритм"})
		progress.Report(ctxOptimized, "variant", "Рассчитываем оптимизированный алгоритм")
		optimized, err := s.Plans.Build(ctxOptimized, c.BuildPlanRequest{
			RequestID: "comparison-optimized", ScenarioID: scenarioID,
			SnapshotRevision: root.SnapshotRevision, SolveMode: c.SolveModeOptimized,
		})
		if err != nil {
			return nil, err
		}
		progress.Report(progress.WithState(ctx, progress.State{Completed: 2, Total: 2, Label: "Оптимизированный алгоритм"}), "variant_complete", "Сравнение готово")
		return map[string]any{
			"snapshot":  source.Snapshot,
			"baseline":  c.Plan{ID: "comparison-baseline", PlanDraft: baseline.Draft},
			"optimized": c.Plan{ID: "comparison-optimized", PlanDraft: optimized.Draft},
		}, nil
	})
}
