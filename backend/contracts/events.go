package contracts

import (
	"encoding/json"
	"time"
)

// EventPayload is a typed helper for Go-4. Event itself uses the canonical wire representation.
type EventPayload struct {
	Events        []Event        `json:"events,omitempty"`
	Order         *Order         `json:"order,omitempty"`
	Location      *LocationInput `json:"location,omitempty"`
	OrderID       string         `json:"order_id,omitempty"`
	OrderIDs      []string       `json:"order_ids,omitempty"`
	EngineerID    string         `json:"engineer_id,omitempty"`
	Status        OrderStatus    `json:"status,omitempty"`
	Reason        string         `json:"reason,omitempty"`
	ExpectedEndAt *time.Time     `json:"expected_end_at,omitempty"`
}

func EncodePayload(p EventPayload) json.RawMessage { b, _ := json.Marshal(p); return b }
func DecodePayload(b json.RawMessage) EventPayload {
	var p EventPayload
	json.Unmarshal(b, &p)
	return p
}
