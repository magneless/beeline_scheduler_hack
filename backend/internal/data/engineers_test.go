package data_test

import (
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"strings"
	"testing"
)

const engineerHeader = "id;skills;transport;shift_start;shift_end;available;router;tv_box\n"

func TestParseEngineersStrictAndStable(t *testing.T) {
	raw := engineerHeader + "e1;fiber, tv;car;08:00;17:00;true;1;2\n" + "e2;;walk;09:00;18:00;false;0;0\n"
	got, err := data.ParseEngineers(strings.NewReader(raw), "2026-08-17", "Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || len(got[1].Skills) != 0 {
		t.Fatalf("unexpected roster: %#v", got)
	}
	for _, tc := range []string{"1", "TRUE", "yes"} {
		_, err = data.ParseEngineers(strings.NewReader(engineerHeader+"e1;;car;08:00;17:00;"+tc+";0;0\n"), "2026-08-17", "Europe/Moscow")
		if err == nil {
			t.Errorf("available=%q accepted", tc)
		}
	}
}
func TestParseEngineersRejectsDuplicateEmptyAndOversize(t *testing.T) {
	dup := engineerHeader + "e1;;car;08:00;17:00;true;0;0\ne1;;car;08:00;17:00;true;0;0\n"
	if _, err := data.ParseEngineers(strings.NewReader(dup), "2026-08-17", "Europe/Moscow"); err == nil {
		t.Fatal("duplicate accepted")
	}
	if _, err := data.ParseEngineers(strings.NewReader(engineerHeader), "2026-08-17", "Europe/Moscow"); err == nil {
		t.Fatal("empty roster accepted")
	}
	huge := engineerHeader + "e1;" + strings.Repeat("x", 10*1024*1024) + ";car;08:00;17:00;true;0;0\n"
	if _, err := data.ParseEngineers(strings.NewReader(huge), "2026-08-17", "Europe/Moscow"); err == nil {
		t.Fatal("oversize roster accepted")
	}
}
