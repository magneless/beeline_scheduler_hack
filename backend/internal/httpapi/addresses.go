package httpapi

import (
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"net/http"
	"strings"
)

func (s *Server) searchAddress(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Address  string `json:"address"`
		RegionID string `json:"region_id"`
	}
	if err := decode(r, &input); err != nil {
		failure(w, err)
		return
	}
	if strings.TrimSpace(input.Address) == "" || len(input.Address) > 1000 {
		failure(w, invalid("Укажите адрес длиной до 1000 символов"))
		return
	}
	result, err := s.Importer.Geo.Geocode(r.Context(), c.GeocodeRequest{RegionID: input.RegionID, Locations: []c.LocationInput{{ID: "address-search", Address: input.Address}}})
	if err != nil {
		failure(w, err)
		return
	}
	if len(result.Items) != 1 {
		failure(w, c.NewError("GEO_UNAVAILABLE", "Некорректный ответ геокодера"))
		return
	}
	write(w, 200, result.Items[0])
}

func (s *Server) resolveOrderAddress(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Revision int64    `json:"snapshot_revision"`
		Address  string   `json:"address"`
		Point    *c.Point `json:"point"`
	}
	if err := decode(r, &input); err != nil {
		failure(w, err)
		return
	}
	if input.Point == nil || input.Revision < 1 {
		failure(w, invalid("Требуются snapshot_revision и подтверждённые координаты"))
		return
	}
	view, err := s.Store.ResolveOrderAddress(r.Context(), r.PathValue("id"), r.PathValue("order_id"), input.Revision, c.Location{Address: input.Address, Point: *input.Point})
	respond(w, 200, view, err)
}
