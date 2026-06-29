package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/biairmal/go-sdk/repository"
)

// KeyGenerator interface will:
// 1. Generate deterministic cache keys for entities
// 2. Generate keys from filter options for list caching
// 3. Include namespace to avoid collisions
// 4. Hash complex filters for shorter keys
//
// See specs/REPOSITORY_SPEC.md section 9 for detailed specification.
type KeyGenerator interface {
	Generate(prefix string, id any) string
	GenerateFromFilter(opts repository.ListOptions) string
}

type DefaultKeyGenerator struct {
	namespace string // e.g., "user", "order"
}

func NewDefaultKeyGenerator(namespace string) KeyGenerator {
	return &DefaultKeyGenerator{namespace: namespace}
}

func (g *DefaultKeyGenerator) Generate(prefix string, id any) string {
	return fmt.Sprintf("%s:%s:%v", g.namespace, prefix, id)
	// Example: "user:id:123"
}

// GenerateFromFilter hashes the filter options to create a consistent key.
// Example: "user:list:a3f5c2d1e8b4c7f0:limit:20:offset:0"
func (g *DefaultKeyGenerator) GenerateFromFilter(opts repository.ListOptions) string {
	hash := hashFilter(opts.Filter, opts.Sorts)
	return fmt.Sprintf("%s:list:%s:limit:%d:offset:%d",
		g.namespace, hash, opts.Pagination.Limit, opts.Pagination.Offset)
}

func hashFilter(filter repository.Filter, sorts []repository.Sort) string {
	normalizedConditions := normalizeConditions(filter.Conditions)
	normalizedSorts := normalizeSorts(sorts)

	payload := struct {
		Conditions []repository.FilterCondition `json:"conditions"`
		Sorts      []repository.Sort            `json:"sorts"`
	}{
		Conditions: normalizedConditions,
		Sorts:      normalizedSorts,
	}

	b, err := json.Marshal(payload)
	if err != nil {
		// json.Marshal failed (e.g. filter Value holds an unmarshalable type like chan or func).
		// Fall back to hashing the raw string representation so that distinct filters that both
		// fail to marshal do not collide on the same constant key.
		raw := sha256.Sum256([]byte(fmt.Sprintf("%v\x00%v", filter.Conditions, sorts)))
		return hex.EncodeToString(raw[:])[:16]
	}

	hash := sha256.Sum256(b)

	// Short hash for cache key readability
	return hex.EncodeToString(hash[:])[:16]
}

func normalizeConditions(conditions []repository.FilterCondition) []repository.FilterCondition {
	out := make([]repository.FilterCondition, len(conditions))
	copy(out, conditions)

	for i := range out {
		// normalize field casing/spacing
		out[i].Field = strings.TrimSpace(strings.ToLower(out[i].Field))

		// sort IN values so order does not matter
		if out[i].Operator == repository.FilterOperatorIn {
			sort.Slice(out[i].Values, func(a, b int) bool {
				return fmt.Sprintf("%v", out[i].Values[a]) < fmt.Sprintf("%v", out[i].Values[b])
			})
		}
	}

	// sort conditions for deterministic ordering
	sort.Slice(out, func(i, j int) bool {
		a := out[i]
		b := out[j]

		if a.Field != b.Field {
			return a.Field < b.Field
		}

		if a.Operator != b.Operator {
			return a.Operator < b.Operator
		}

		// Use a null-byte separator to avoid fmt.Sprint's space-insertion rule, which
		// produces the same output for (int(1), nil) and (string("1 "), nil).
		return fmt.Sprintf("%v\x00%v", a.Value, a.Values) <
			fmt.Sprintf("%v\x00%v", b.Value, b.Values)
	})

	return out
}

func normalizeSorts(sorts []repository.Sort) []repository.Sort {
	out := make([]repository.Sort, len(sorts))
	copy(out, sorts)

	for i := range out {
		out[i].Field = strings.TrimSpace(strings.ToLower(out[i].Field))
	}

	return out
}
