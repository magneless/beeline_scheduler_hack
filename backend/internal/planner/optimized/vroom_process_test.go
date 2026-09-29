package optimized

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
)

// The test executable acts as a controlled native process. This exercises
// stdin/stdout, real cancellation and Wait without installing VROOM.
func TestMain(m *testing.M) {
	if mode := os.Getenv("LCT_VROOM_PROCESS_HELPER"); mode != "" {
		if path := os.Getenv("LCT_VROOM_HELPER_PID"); path != "" {
			if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
				os.Exit(2)
			}
		}
		if mode == "slow" {
			time.Sleep(30 * time.Second)
		}
		if mode == "fail" {
			fmt.Fprint(os.Stderr, "native failure")
			os.Exit(1)
		}
		var input vroomInput
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			os.Exit(2)
		}
		if len(os.Args) != 7 || os.Args[1] != "-t" || os.Args[2] != "2" || os.Args[3] != "-x" || os.Args[4] != "5" || os.Args[5] != "-l" {
			os.Exit(3)
		}
		code := 0
		out := vroomOutput{Code: &code, Unassigned: []vroomStep{}, Routes: []vroomRoute{{Vehicle: 1, Steps: []vroomStep{}}}}
		for _, job := range input.Jobs {
			out.Routes[0].Steps = append(out.Routes[0].Steps, vroomStep{Type: "job", ID: job.ID})
		}
		if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func helperExecutable(t *testing.T, mode string) string {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LCT_VROOM_PROCESS_HELPER", mode)
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	return binary
}

func TestVROOMProcessRoundTripAndFailure(t *testing.T) {
	p := modelProblem(t, testutil.BaseRequest())
	input, err := p.vroomRequest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	binary := helperExecutable(t, "success")
	data, err := runVROOM(context.Background(), binary, input, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	seq, err := p.vroomSequences(input, data)
	if err != nil || len(seq[0]) != 1 || seq[0][0] != 0 {
		t.Fatalf("process round trip: %v %v", seq, err)
	}
	t.Setenv("LCT_VROOM_PROCESS_HELPER", "fail")
	_, err = runVROOM(context.Background(), binary, input, time.Now().Add(time.Second))
	var contract *contracts.ContractError
	if !errors.As(err, &contract) || contract.Code != "COMPUTATION_FAILED" {
		t.Fatalf("native failure was hidden: %v", err)
	}
}

func TestVROOMCancellationKillsAndReapsProcess(t *testing.T) {
	binary := helperExecutable(t, "slow")
	pidFile := filepath.Join(t.TempDir(), "pid")
	t.Setenv("LCT_VROOM_HELPER_PID", pidFile)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := runVROOM(ctx, binary, vroomInput{}, time.Now().Add(5*time.Second))
		done <- err
	}()
	var pidData []byte
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		pidData, _ = os.ReadFile(pidFile)
		if len(pidData) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(pidData) == 0 {
		t.Fatal("helper did not start")
	}
	started := time.Now()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) || time.Since(started) > time.Second {
		t.Fatalf("cancellation must stop promptly: %v", err)
	}
	if runtime.GOOS == "linux" {
		pid, _ := strconv.Atoi(string(pidData))
		process, err := os.FindProcess(pid)
		if err != nil {
			t.Fatal(err)
		}
		defer process.Release()
		if err := process.Signal(syscall.Signal(0)); !errors.Is(err, syscall.ESRCH) && !errors.Is(err, os.ErrProcessDone) {
			t.Fatalf("child process %d still exists after cancellation: %v", pid, err)
		}
	}
}

func TestVROOMInternalTimeoutReturnsValidIncumbent(t *testing.T) {
	binary := helperExecutable(t, "slow")
	in := testutil.BaseRequest()
	in.Mode, in.TimeLimitMS = contracts.SolveModeOptimized, 100
	got, err := (&Optimized{binary: binary}).Solve(context.Background(), in)
	if err != nil || got.Termination != contracts.TerminationTimeLimit {
		t.Fatalf("internal budget must return an incumbent: %+v %v", got, err)
	}
	if len(testutil.AssignedIDs(got))+len(got.Unassigned) != len(in.Orders) {
		t.Fatalf("orders were lost at timeout: %+v", got)
	}
}

func TestVROOMMissingExecutableIsExplicit(t *testing.T) {
	in := testutil.BaseRequest()
	in.Mode = contracts.SolveModeOptimized
	got, err := (&Optimized{binary: filepath.Join(t.TempDir(), "missing-vroom")}).Solve(context.Background(), in)
	var contract *contracts.ContractError
	if !errors.As(err, &contract) || contract.Code != "COMPUTATION_FAILED" || len(got.Routes) != 0 {
		t.Fatalf("missing executable must not silently substitute another solver: %+v %v", got, err)
	}
}
