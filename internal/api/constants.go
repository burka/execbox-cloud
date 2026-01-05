package api

// Session status constants
const (
	SessionStatusPending = "pending"
	SessionStatusRunning = "running"
	SessionStatusStopped = "stopped"
	SessionStatusKilled  = "killed"
	SessionStatusFailed  = "failed"
)

// Tier constants
const (
	TierAnonymous  = "anonymous"
	TierFree       = "free"
	TierStarter    = "starter"
	TierPro        = "pro"
	TierEnterprise = "enterprise"
)

// Quota request status constants
const (
	QuotaStatusPending   = "pending"
	QuotaStatusContacted = "contacted"
	QuotaStatusConverted = "converted"
	QuotaStatusDeclined  = "declined"
)
