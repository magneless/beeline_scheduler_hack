package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/testkit"
)

func TestProposalEndpointStreamsSavedResult(t *testing.T) {
	_, store, fixture := integrationServer(t)
	snap := fixture.Snapshot
	snap.ScenarioID = storage.ID("scenario")
	view, err := store.CreateScenario(context.Background(), snap, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	h := (&Server{Store: store, Plans: &optionFixtureService{Plans: &testkit.Plans{Reader: store, Fixture: fixture}}}).Handler()
	body, _ := json.Marshal(map[string]any{"request_id": "stream-proposal", "snapshot_revision": 1, "expected_current_plan_id": nil})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/scenarios/"+view.Snapshot.ScenarioID+"/proposals", bytes.NewReader(body))
	r.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "event: result") || !strings.Contains(w.Body.String(), `"request_id":"stream-proposal"`) {
		t.Fatalf("unexpected streamed proposal: %d %s", w.Code, w.Body.String())
	}
	if _, found, err := store.ProposalByRequest(context.Background(), view.Snapshot.ScenarioID, "stream-proposal"); err != nil || !found {
		t.Fatalf("proposal not saved: found=%v err=%v", found, err)
	}
}

func TestCalculationStreamProgressAndResult(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Accept", "application/json, text/event-stream")
	w := httptest.NewRecorder()
	respondCalculation(w, r, 201, 2, func(ctx context.Context) (any, error) {
		progress.Report(progress.WithState(ctx, progress.State{Completed: 1, Total: 2, Label: "Вариант 1"}), "variant_complete", "Готов")
		return map[string]any{"id": "proposal-1"}, nil
	})
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("unexpected response: %d %v", w.Code, w.Header())
	}
	body := w.Body.String()
	for _, expected := range []string{"event: progress", `"completed":1`, `"total":2`, `"variant_label":"Вариант 1"`, "event: result", `"id":"proposal-1"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("missing %q in stream: %s", expected, body)
		}
	}
}

func TestCalculationStreamErrorAndJSONCompatibility(t *testing.T) {
	makeFailure := func(context.Context) (any, error) {
		return nil, c.NewError("GEO_UNAVAILABLE", "Нет маршрута")
	}
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	respondCalculation(w, r, 201, 2, makeFailure)
	if !strings.Contains(w.Body.String(), "event: error") || !strings.Contains(w.Body.String(), `"code":"GEO_UNAVAILABLE"`) {
		t.Fatal(w.Body.String())
	}
	r = httptest.NewRequest(http.MethodPost, "/", nil)
	w = httptest.NewRecorder()
	respondCalculation(w, r, 201, 2, makeFailure)
	if w.Code != 503 || strings.Contains(w.Body.String(), "event:") {
		t.Fatalf("JSON error behavior changed: %d %s", w.Code, w.Body.String())
	}
}

func TestCalculationStreamHeartbeatAndCancellation(t *testing.T) {
	old := calculationHeartbeatInterval
	calculationHeartbeatInterval = 10 * time.Millisecond
	t.Cleanup(func() { calculationHeartbeatInterval = old })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	r.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	workerDone := make(chan struct{})
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		respondCalculation(w, r, 201, 2, func(ctx context.Context) (any, error) {
			defer close(workerDone)
			<-ctx.Done()
			return nil, ctx.Err()
		})
	}()
	time.Sleep(40 * time.Millisecond)
	// Recorder is read only once its writer has stopped.
	cancel()
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("stream did not stop on disconnect")
	}
	select {
	case <-workerDone:
	case <-time.After(time.Second):
		t.Fatal("calculation was not cancelled")
	}
	if !strings.Contains(w.Body.String(), "event: heartbeat") {
		t.Fatalf("missing heartbeat: %s", w.Body.String())
	}
}
