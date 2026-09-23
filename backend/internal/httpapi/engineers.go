package httpapi

import (
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"net/http"
	"strconv"
)

func (s *Server) importEngineers(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1024 * 1024); err != nil {
		failure(w, &responseError{400, "Некорректный multipart"})
		return
	}
	defer r.MultipartForm.RemoveAll()
	raw := r.FormValue("expected_revision")
	rev, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || rev < 1 {
		failure(w, invalid("Требуется положительный expected_revision"))
		return
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		failure(w, &responseError{400, "Отсутствует file"})
		return
	}
	defer f.Close()
	v, err := s.Store.GetScenario(r.Context(), r.PathValue("id"), 0)
	if err != nil {
		failure(w, err)
		return
	}
	engineers, err := data.ParseEngineers(f, v.Snapshot.Date, v.Snapshot.Timezone)
	if err != nil {
		failure(w, err)
		return
	}
	updated, err := s.Store.ReplaceEngineers(r.Context(), r.PathValue("id"), rev, engineers)
	respond(w, 200, updated, err)
}
