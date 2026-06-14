package quality

import (
	"fmt"
	"slices"
)

// ValidLevel reports whether level is a known gate level.
func ValidLevel(level string) bool {
	return slices.Contains(LevelOrder, level)
}

// LevelRank returns the sort order for a level (higher = run later).
func LevelRank(level string) int {
	for i, l := range LevelOrder {
		if l == level {
			return i
		}
	}
	return len(LevelOrder)
}

// SortGatesByLevel sorts gates by canonical level order, preserving name order
// within the same level.
func SortGatesByLevel(gates []Gate) {
	slices.SortFunc(gates, func(a, b Gate) int {
		if r := LevelRank(a.Level) - LevelRank(b.Level); r != 0 {
			return r
		}
		return stringsCompare(a.Name, b.Name)
	})
}

func stringsCompare(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// ValidateGateNames ensures gate names are unique.
func ValidateGateNames(gates []Gate) error {
	seen := make(map[string]bool, len(gates))
	for _, g := range gates {
		if g.Name == "" {
			return fmt.Errorf("quality gate missing name")
		}
		if seen[g.Name] {
			return fmt.Errorf("duplicate quality gate name %q", g.Name)
		}
		seen[g.Name] = true
	}
	return nil
}
