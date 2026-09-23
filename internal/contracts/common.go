package contracts

// Location represents a resolved geographic location with coordinates.
type Location struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Point   Point  `json:"point"`
}

// LocationInput represents an input location that may require geocoding.
// If Point is nil, the address must be geocoded to obtain coordinates.
type LocationInput struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Point   *Point `json:"point"`
}

// Issue represents a problem encountered during data processing.
type Issue struct {
	SourceRow *int    `json:"source_row"`
	EntityID  *string `json:"entity_id"`
	Field     *string `json:"field"`
	Code      string  `json:"code"`
	Message   string  `json:"message"`
}
