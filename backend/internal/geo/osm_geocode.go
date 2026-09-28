package geo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

type geocodeInputError struct{ message string }

func (e *geocodeInputError) Error() string { return e.message }

type photonFeatureResult struct {
	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		Street   string    `json:"street"`
		House    string    `json:"housenumber"`
		City     string    `json:"city"`
		Locality string    `json:"locality"`
		District string    `json:"district"`
		County   string    `json:"county"`
		State    string    `json:"state"`
		Name     string    `json:"name"`
		OSMType  string    `json:"osm_type"`
		OSMID    int64     `json:"osm_id"`
		OSMKey   string    `json:"osm_key"`
		Extent   []float64 `json:"extent"`
	} `json:"properties"`
}
type photonResponse struct {
	Type     string                `json:"type"`
	Features []photonFeatureResult `json:"features"`
}
type photonSearch struct {
	path     string
	params   url.Values
	optional bool
}

func photonSearches(address string, parts russianAddress) []photonSearch {
	common := func() url.Values {
		return url.Values{"countrycode": {"RU"}, "limit": {"50"}, "layer": {"house"}, "dedupe": {"0"}}
	}
	text := common()
	normalized := strings.TrimSpace(parts.city + " " + parts.street + " " + parts.house)
	if !parts.ok {
		normalized = strings.TrimSpace(address)
	}
	text.Set("q", normalized)
	searches := []photonSearch{{path: "/api/", params: text}}
	if parts.ok {
		structured := common()
		structured.Set("city", parts.city)
		structured.Set("street", parts.street)
		if parts.state != "" {
			structured.Set("state", parts.state)
		}
		// Search the base number to retrieve compound houses unsupported by the
		// structured endpoint. Acceptance still checks the ENTIRE requested house.
		house := strings.FieldsFunc(parts.house, func(r rune) bool { return r == ' ' || r == '/' || r == '-' })[0]
		structured.Set("housenumber", house)
		searches = append(searches, photonSearch{path: "/structured", params: structured, optional: true})
	}
	original := strings.TrimSpace(address)
	if original != normalized {
		raw := common()
		raw.Set("q", original)
		searches = append(searches, photonSearch{path: "/api/", params: raw})
	}
	return searches
}

func (p *OSMProvider) Geocode(ctx context.Context, address string) ([]contracts.Location, error) {
	result, err := p.SearchAddress(ctx, address)
	locations := make([]contracts.Location, 0, len(result.Exact))
	for _, candidate := range result.Exact {
		locations = append(locations, contracts.Location{Address: address, Point: candidate.Point})
	}
	return locations, err
}

// SearchAddress asks the full-text index even when our normalization cannot
// decompose the input. Normalization generates hints; it is not an input gate.
func (p *OSMProvider) SearchAddress(ctx context.Context, address string) (AddressSearchResult, error) {
	if err := ctx.Err(); err != nil {
		return AddressSearchResult{}, err
	}
	if strings.TrimSpace(address) == "" {
		return AddressSearchResult{}, &geocodeInputError{message: "Укажите адрес или координаты"}
	}
	parts := parseRussianAddress(address)
	var suggestions []photonFeatureResult
	for _, search := range photonSearches(address, parts) {
		endpoint := strings.TrimRight(p.o.PhotonURL, "/") + search.path + "?" + search.params.Encode()
		var response photonResponse
		if err := p.get(ctx, endpoint, &response); err != nil {
			var status *osmHTTPError
			if search.optional && errors.As(err, &status) && (status.status == http.StatusNotFound || status.status == http.StatusBadRequest || status.status == http.StatusUnprocessableEntity) {
				continue
			}
			return AddressSearchResult{}, err
		}
		if response.Type != "FeatureCollection" || response.Features == nil {
			return AddressSearchResult{}, fmt.Errorf("malformed Photon response")
		}
		var exact []photonFeatureResult
		for _, feature := range response.Features {
			xy := feature.Geometry.Coordinates
			if feature.Geometry.Type != "Point" || len(xy) != 2 || !validPoint(xy[1], xy[0]) {
				return AddressSearchResult{}, fmt.Errorf("invalid Photon coordinates")
			}
			props := feature.Properties
			if normHouse(props.House) == "" || props.Street == "" {
				continue
			}
			if candidateMatches(address, parts, feature) {
				exact = append(exact, feature)
			}
			// Suggestions must be real house objects, never the centre of a street.
			if candidateLocalityMatches(address, parts, feature) {
				suggestions = append(suggestions, feature)
			}
		}
		buildings := []photonFeatureResult{}
		for _, feature := range exact {
			if feature.Properties.OSMKey == "building" {
				buildings = append(buildings, feature)
			}
		}
		// The request is for a house, so its footprint is more authoritative
		// than an organisation whose postal address may be stale or incomplete.
		if len(buildings) > 0 {
			exact = buildings
		}
		exact = dedupeAddressObjects(exact)
		if len(exact) > 0 {
			return AddressSearchResult{Exact: addressCandidates(exact), Suggestions: addressCandidates(exact)}, nil
		}
	}
	suggestions = dedupeAddressObjects(suggestions)
	score := func(f photonFeatureResult) int {
		value := 0
		name, _ := streetParts(f.Properties.Street)
		wanted, _ := streetParts(parts.street)
		if wanted != "" && name == wanted {
			value += 10
		}
		base := strings.FieldsFunc(parts.house, func(r rune) bool { return r == ' ' || r == '/' })
		house := strings.FieldsFunc(normHouse(f.Properties.House), func(r rune) bool { return r == ' ' || r == '/' })
		if len(base) > 0 && len(house) > 0 && base[0] == house[0] {
			value += 5
		}
		if f.Properties.OSMKey == "building" {
			value++
		}
		return value
	}
	sort.SliceStable(suggestions, func(i, j int) bool { return score(suggestions[i]) > score(suggestions[j]) })
	if len(suggestions) > 12 {
		suggestions = suggestions[:12]
	}
	return AddressSearchResult{Suggestions: addressCandidates(suggestions)}, nil
}

func candidateLocalityMatches(raw string, wanted russianAddress, f photonFeatureResult) bool {
	props := f.Properties
	if wanted.state != "" && normAddressPart(props.State) != wanted.state {
		return false
	}
	for _, place := range []string{props.City, props.Locality, props.District} {
		place = normAddressPart(place)
		if place == "" {
			continue
		}
		if wanted.city != "" && wanted.city == place {
			return true
		}
		if containsWords(normAddressPart(raw), place) {
			return true
		}
	}
	return false
}
func candidateMatches(raw string, wanted russianAddress, f photonFeatureResult) bool {
	if !candidateLocalityMatches(raw, wanted, f) {
		return false
	}
	props := f.Properties
	if wanted.ok && addressMatches(raw, wanted, wanted.city, props.Street, props.House) {
		return true
	}
	// Candidate-guided parsing: use the returned street vocabulary to recognize
	// unusual punctuation/order without inventing a settlement or building suffix.
	requestedHouse := houseTail(raw)
	if normHouse(requestedHouse) == "" || normHouse(requestedHouse) != normHouse(props.House) {
		return false
	}
	if unit := unitMarkerRE.FindStringIndex(raw); unit != nil {
		raw = raw[:unit[0]]
	}
	m := houseMarkerRE.FindStringSubmatchIndex(raw)
	if m == nil {
		m = houseTailRE.FindStringSubmatchIndex(raw)
	}
	if m == nil {
		return false
	}
	words := strings.Fields(normAddressPart(raw[:m[0]]))
	remaining := map[string]int{}
	for _, word := range words {
		remaining[normalizeOrdinal(word)]++
	}
	street, kind := streetParts(props.Street)
	for _, word := range strings.Fields(street) {
		if remaining[word] == 0 {
			return false
		}
		remaining[word]--
	}
	// Explicit street types cannot be silently changed or ignored.
	for _, typ := range []string{"улица", "проспект", "проезд", "переулок", "шоссе", "площадь", "набережная", "бульвар"} {
		if remaining[typ] > 0 && typ != kind {
			return false
		}
		delete(remaining, typ)
	}
	for _, part := range []string{props.City, props.Locality, props.District, props.County, props.State} {
		for _, word := range strings.Fields(normAddressPart(part)) {
			delete(remaining, normalizeOrdinal(word))
		}
	}
	for word, count := range remaining {
		if count <= 0 {
			continue
		}
		switch word {
		case "россия", "рф", "обл", "город", "поселок", "поселение", "пгт", "деревня", "село", "р-н", "район":
			continue
		}
		if len(word) == 6 && allDigits(word) {
			continue
		}
		return false
	}
	return true
}
func containsWords(text, part string) bool { return strings.Contains(" "+text+" ", " "+part+" ") }
func houseTail(raw string) string {
	if unit := unitMarkerRE.FindStringIndex(raw); unit != nil {
		raw = raw[:unit[0]]
	}
	if m := houseMarkerRE.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	if m := houseTailRE.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	return ""
}

// Merge duplicate representations of a building (POI/entrance/building) only
// with spatial evidence. Distinct buildings sharing an address stay selectable.
func dedupeAddressObjects(features []photonFeatureResult) []photonFeatureResult {
	sort.SliceStable(features, func(i, j int) bool {
		return features[i].Properties.OSMKey == "building" && features[j].Properties.OSMKey != "building"
	})
	out := []photonFeatureResult{}
	for _, f := range features {
		duplicate := false
		for _, kept := range out {
			a, b := f.Properties, kept.Properties
			if normHouse(a.House) != normHouse(b.House) || normAddressPart(a.Street) != normAddressPart(b.Street) || normAddressPart(a.City) != normAddressPart(b.City) {
				continue
			}
			xy, kxy := f.Geometry.Coordinates, kept.Geometry.Coordinates
			sameID := a.OSMID != 0 && a.OSMID == b.OSMID && a.OSMType == b.OSMType
			inside := b.OSMKey == "building" && a.OSMKey != "building" && len(b.Extent) == 4 && xy[0] >= math.Min(b.Extent[0], b.Extent[2]) && xy[0] <= math.Max(b.Extent[0], b.Extent[2]) && xy[1] >= math.Min(b.Extent[1], b.Extent[3]) && xy[1] <= math.Max(b.Extent[1], b.Extent[3])
			if sameID || inside || (xy[0] == kxy[0] && xy[1] == kxy[1]) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, f)
		}
	}
	return out
}
func addressCandidates(features []photonFeatureResult) []contracts.AddressCandidate {
	out := []contracts.AddressCandidate{}
	for _, f := range features {
		p := f.Properties
		city := p.City
		if city == "" {
			city = p.Locality
		}
		if city == "" {
			city = p.District
		}
		source := ""
		kind := map[string]string{"N": "node", "W": "way", "R": "relation"}[p.OSMType]
		if kind != "" && p.OSMID > 0 {
			source = fmt.Sprintf("https://www.openstreetmap.org/%s/%d", kind, p.OSMID)
		}
		out = append(out, contracts.AddressCandidate{Address: strings.Join([]string{city, p.Street, p.House}, ", "), Point: contracts.Point{Lat: f.Geometry.Coordinates[1], Lon: f.Geometry.Coordinates[0]}, Source: source})
	}
	return out
}
