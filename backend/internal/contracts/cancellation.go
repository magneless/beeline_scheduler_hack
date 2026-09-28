package contracts

import "strings"

// IDs validates both the single-order wire format and an atomic selection.
// Batch cancellation is restricted to unassigned work by the plan service.
func (p OrderCancelled) IDs() ([]string, error) {
	if p.Reason != "client_refusal" && p.Reason != "cannot_perform" {
		return nil, NewError("INVALID_INPUT", "Укажите допустимую причину отмены")
	}
	ids := p.OrderIDs
	if p.OrderIDs == nil && p.OrderID != "" {
		ids = []string{p.OrderID}
	} else if p.OrderID != "" || len(p.OrderIDs) == 0 {
		return nil, NewError("INVALID_INPUT", "Укажите order_id или непустой список order_ids")
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if strings.TrimSpace(id) == "" || seen[id] {
			return nil, NewError("INVALID_INPUT", "Список заявок содержит пустой ID или повтор")
		}
		seen[id] = true
	}
	return ids, nil
}
