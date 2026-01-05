package api

// TierLimits defines quota limits for each subscription tier.
type TierLimits struct {
	SessionsPerDay     int // -1 means unlimited
	ConcurrentSessions int
	MaxDurationSec     int
	MemoryMB           int
}

// tierLimits maps tier names to their quota limits.
var tierLimits = map[string]TierLimits{
	TierAnonymous: {
		SessionsPerDay:     3,
		ConcurrentSessions: 1,
		MaxDurationSec:     60,
		MemoryMB:           512,
	},
	TierFree: {
		SessionsPerDay:     10,
		ConcurrentSessions: 5,
		MaxDurationSec:     60,
		MemoryMB:           512,
	},
	TierStarter: {
		SessionsPerDay:     100,
		ConcurrentSessions: 10,
		MaxDurationSec:     300,
		MemoryMB:           1024,
	},
	TierPro: {
		SessionsPerDay:     1000,
		ConcurrentSessions: 50,
		MaxDurationSec:     600,
		MemoryMB:           2048,
	},
	TierEnterprise: {
		SessionsPerDay:     -1, // unlimited
		ConcurrentSessions: -1, // unlimited
		MaxDurationSec:     -1, // unlimited
		MemoryMB:           -1, // unlimited
	},
}

// GetTierLimits returns the quota limits for a given tier.
// If the tier is unknown, it returns limits for the free tier as a safe default.
func GetTierLimits(tier string) TierLimits {
	limits, ok := tierLimits[tier]
	if !ok {
		return tierLimits[TierFree]
	}
	return limits
}

// IsUnlimited checks if a limit value represents unlimited quota.
func IsUnlimited(limit int) bool {
	return limit < 0
}
