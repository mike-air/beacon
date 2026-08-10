package search

// Request and Response are the engine-neutral shapes the service speaks, so
// internal/search does not import the Meilisearch SDK and the engine can be
// swapped without touching the service. [glue, implied by ch30's Indexer
// boundary.]
type Request struct {
	Query                 string
	Filter                string
	Limit                 int
	Offset                int
	AttributesToHighlight []string
}

type Response struct {
	Hits []RawHit
}

// RawHit keeps the whole document around so Highlighted can prefer the
// engine's marked-up version of a field and fall back to the plain one.
type RawHit struct {
	Kind     string
	EntityID string
	Fields   map[string]any
}

// Highlighted returns the field with <mark> tags around the matched terms when
// Meili produced them, and the plain value otherwise.
func (h RawHit) Highlighted(field string) string {
	if fm, ok := h.Fields["_formatted"].(map[string]any); ok {
		if v, ok := fm[field].(string); ok {
			return v
		}
	}
	if v, ok := h.Fields[field].(string); ok {
		return v
	}
	return ""
}
