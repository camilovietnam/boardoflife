// Package healthbars implements the per-user health bars that gamify each
// life area: validation rules, the persistence port and the HTTP handlers.
package healthbars

import (
	"context"
	"math"
	"slices"
	"time"
)

// Life areas that have a health bar. This whitelist is the single source of
// truth; the database does not constrain the section.
var sections = []string{"books", "exercise", "learning", "mindfulness"}

const (
	// MaxHealth is the full-health value, in points.
	MaxHealth = 10
	// halfPointsPerPoint: health is stored and exchanged with the Store in
	// half-points (0..20) so that steps of 0.5 are exact integers.
	halfPointsPerPoint = 2
	maxHalfPoints      = MaxHealth * halfPointsPerPoint
)

// ValidSection reports whether s is a known life area.
func ValidSection(s string) bool { return slices.Contains(sections, s) }

// Record is a stored health bar. Health is in half-points (0..20).
type Record struct {
	HalfPoints  int
	LastUpdated time.Time
}

// Store is the persistence port of the healthbars package. Every method is
// scoped to userID; section must already be validated with ValidSection.
type Store interface {
	// Get returns the bar, first creating it at full health when the user
	// has none for that section yet.
	Get(ctx context.Context, userID int64, section string) (Record, error)
	// Set stores an absolute health value (upsert) and refreshes LastUpdated.
	Set(ctx context.Context, userID int64, section string, halfPoints int) (Record, error)
}

// Healthbar is a health bar as exposed over the API.
type Healthbar struct {
	Health      float64   `json:"health"`
	Max         int       `json:"max"`
	LastUpdated time.Time `json:"last_updated"`
}

func toHealthbar(r Record) Healthbar {
	return Healthbar{
		Health:      float64(r.HalfPoints) / halfPointsPerPoint,
		Max:         MaxHealth,
		LastUpdated: r.LastUpdated,
	}
}

// FieldErrors maps a JSON field name to a human readable problem.
type FieldErrors map[string]string

func (e FieldErrors) Error() string { return "validation failed" }

// Input is the body of PUT /healthbars/{section}.
type Input struct {
	Health *float64 `json:"health"`
}

// Validate returns the requested health in half-points. On failure the
// returned error is a non-empty FieldErrors.
func (in Input) Validate() (int, error) {
	const problem = "must be a number between 0 and 10 in steps of 0.5"
	if in.Health == nil {
		return 0, FieldErrors{"health": "is required"}
	}
	half := *in.Health * halfPointsPerPoint
	if math.IsNaN(half) || half < 0 || half > maxHalfPoints || half != math.Trunc(half) {
		return 0, FieldErrors{"health": problem}
	}
	return int(half), nil
}
