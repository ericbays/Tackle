package smtp

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"sync/atomic"

	"tackle/internal/repositories"
)

// Target represents the email recipient used by segment-aware strategies.
type Target struct {
	// Attributes are arbitrary key-value pairs from target enrichment (e.g. department, role).
	Attributes map[string]string
}

// SendingStrategy selects which SMTP profile to use for a given send operation.
type SendingStrategy interface {
	SelectProfile(target Target, profiles []repositories.CampaignSMTPProfile) (repositories.CampaignSMTPProfile, error)
}

// --- Round Robin ---

// RoundRobinStrategy cycles through profiles sequentially using a thread-safe atomic counter.
type RoundRobinStrategy struct {
	counter atomic.Uint64
}

// NewRoundRobinStrategy creates a RoundRobinStrategy.
func NewRoundRobinStrategy() *RoundRobinStrategy { return &RoundRobinStrategy{} }

// SelectProfile returns profiles[counter % len(profiles)] and increments the counter.
func (s *RoundRobinStrategy) SelectProfile(_ Target, profiles []repositories.CampaignSMTPProfile) (repositories.CampaignSMTPProfile, error) {
	if len(profiles) == 0 {
		return repositories.CampaignSMTPProfile{}, fmt.Errorf("round_robin: no profiles available")
	}
	idx := s.counter.Add(1) - 1
	return profiles[idx%uint64(len(profiles))], nil
}

// --- Random ---

// RandomStrategy selects a profile uniformly at random using crypto/rand.
type RandomStrategy struct{}

// NewRandomStrategy creates a RandomStrategy.
func NewRandomStrategy() *RandomStrategy { return &RandomStrategy{} }

// SelectProfile returns a randomly chosen profile.
func (s *RandomStrategy) SelectProfile(_ Target, profiles []repositories.CampaignSMTPProfile) (repositories.CampaignSMTPProfile, error) {
	if len(profiles) == 0 {
		return repositories.CampaignSMTPProfile{}, fmt.Errorf("random: no profiles available")
	}
	idx, err := cryptoRandInt(len(profiles))
	if err != nil {
		return repositories.CampaignSMTPProfile{}, fmt.Errorf("random: %w", err)
	}
	return profiles[idx], nil
}

// --- Weighted ---

// WeightedStrategy selects a profile according to configured weights using crypto/rand.
type WeightedStrategy struct{}

// NewWeightedStrategy creates a WeightedStrategy.
func NewWeightedStrategy() *WeightedStrategy { return &WeightedStrategy{} }

// SelectProfile returns a profile according to its weight.
// Weights must sum to 100; profiles with weight 0 are excluded.
func (s *WeightedStrategy) SelectProfile(_ Target, profiles []repositories.CampaignSMTPProfile) (repositories.CampaignSMTPProfile, error) {
	if len(profiles) == 0 {
		return repositories.CampaignSMTPProfile{}, fmt.Errorf("weighted: no profiles available")
	}

	total := 0
	for _, p := range profiles {
		total += p.Weight
	}
	if total == 0 {
		return repositories.CampaignSMTPProfile{}, fmt.Errorf("weighted: all profile weights are zero")
	}

	// Pick a random number in [0, total).
	n, err := cryptoRandInt(total)
	if err != nil {
		return repositories.CampaignSMTPProfile{}, fmt.Errorf("weighted: %w", err)
	}

	cumulative := 0
	for _, p := range profiles {
		cumulative += p.Weight
		if n < cumulative {
			return p, nil
		}
	}
	// Fallback — should not be reached.
	return profiles[len(profiles)-1], nil
}

// ValidateWeights returns an error if the profile weights do not sum to 100.
func ValidateWeights(profiles []repositories.CampaignSMTPProfile) error {
	total := 0
	for _, p := range profiles {
		total += p.Weight
	}
	if total != 100 {
		return fmt.Errorf("weighted strategy: profile weights must sum to 100, got %d", total)
	}
	return nil
}

// --- Failover ---

// FailoverStrategy tries profiles in priority order and advances on failure.
// Thread-safety: the failure state is per-instance; create one per campaign worker.
type FailoverStrategy struct {
	failedIndex int
}

// NewFailoverStrategy creates a FailoverStrategy.
func NewFailoverStrategy() *FailoverStrategy { return &FailoverStrategy{} }

// SelectProfile returns the current active profile (lowest priority index that has not failed).
// Profiles must be sorted by priority ascending (the repository returns them in this order).
func (s *FailoverStrategy) SelectProfile(_ Target, profiles []repositories.CampaignSMTPProfile) (repositories.CampaignSMTPProfile, error) {
	if len(profiles) == 0 {
		return repositories.CampaignSMTPProfile{}, fmt.Errorf("failover: no profiles available")
	}
	if s.failedIndex >= len(profiles) {
		return repositories.CampaignSMTPProfile{}, fmt.Errorf("failover: all profiles exhausted")
	}
	return profiles[s.failedIndex], nil
}

// RecordFailure advances the active index to the next profile.
func (s *FailoverStrategy) RecordFailure() {
	s.failedIndex++
}

// Reset resets the active index back to the primary profile (for recovery checks).
func (s *FailoverStrategy) Reset() {
	s.failedIndex = 0
}

// --- Segment ---

// SegmentStrategy matches the target's attributes against per-profile JSONB segment filters.
type SegmentStrategy struct{}

// NewSegmentStrategy creates a SegmentStrategy.
func NewSegmentStrategy() *SegmentStrategy { return &SegmentStrategy{} }

// SelectProfile returns the first profile whose segment filter matches the target's attributes.
// Filter is a flat JSON object {"key": "value"} — all entries must match (AND logic).
// A profile with a nil segment filter acts as a wildcard/default fallback.
// Returns an error if no profile matches (configuration error).
func (s *SegmentStrategy) SelectProfile(target Target, profiles []repositories.CampaignSMTPProfile) (repositories.CampaignSMTPProfile, error) {
	if len(profiles) == 0 {
		return repositories.CampaignSMTPProfile{}, fmt.Errorf("segment: no profiles available")
	}
	var wildcard *repositories.CampaignSMTPProfile
	for i := range profiles {
		p := profiles[i]
		if p.SegmentFilter == nil {
			wildcard = &profiles[i]
			continue
		}
		if matchSegmentFilter(p.SegmentFilter, target.Attributes) {
			return p, nil
		}
	}
	if wildcard != nil {
		return *wildcard, nil
	}
	return repositories.CampaignSMTPProfile{}, fmt.Errorf("segment: no profile matched target attributes")
}

// matchSegmentFilter parses a JSONB segment filter and checks whether target attributes match.
// The filter format is {"key": "value", ...}; all key-value pairs must match (AND logic).
func matchSegmentFilter(filterJSON []byte, attrs map[string]string) bool {
	var filter map[string]string
	if err := json.Unmarshal(filterJSON, &filter); err != nil || len(filter) == 0 {
		return len(filter) == 0 // empty filter matches all
	}
	for k, v := range filter {
		if attrs[k] != v {
			return false
		}
	}
	return true
}

// --- Effective rate calculator ---

// EffectiveRate returns the most restrictive (lowest non-zero) rate from campaign and profile
// rate limits. A value of 0 means unlimited.
func EffectiveRate(campaignRate int, profileRates []int) int {
	rates := make([]int, 0, 1+len(profileRates))
	if campaignRate > 0 {
		rates = append(rates, campaignRate)
	}
	for _, r := range profileRates {
		if r > 0 {
			rates = append(rates, r)
		}
	}
	if len(rates) == 0 {
		return 0
	}
	min := rates[0]
	for _, r := range rates[1:] {
		if r < min {
			min = r
		}
	}
	return min
}

// --- crypto/rand helpers ---

// cryptoRandInt returns a cryptographically random integer in [0, n).
func cryptoRandInt(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("cryptoRandInt: n must be positive, got %d", n)
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, fmt.Errorf("crypto/rand: %w", err)
	}
	return int(v.Int64()), nil
}

// cryptoRandUint64 returns a cryptographically random uint64.
// Used by other packages in the smtp service group.
func cryptoRandUint64() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("crypto/rand read: %w", err)
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}
