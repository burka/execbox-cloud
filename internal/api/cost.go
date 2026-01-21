package api

// CostCalculator calculates costs for session usage
type CostCalculator struct {
	BaseCostPerRequest    int64 // in cents (0.1 cents = $0.001)
	CPUCostPerSecond      int64 // in cents per CPU-second (0.005 cents = $0.00005)
	MemoryCostPerGBSecond int64 // in cents per GB-second (0.001 cents = $0.00001)
}

// DefaultCostCalculator is the default cost calculator with standard rates
var DefaultCostCalculator = &CostCalculator{
	BaseCostPerRequest:    1, // 0.1 cents = $0.001
	CPUCostPerSecond:      5, // 0.005 cents = $0.00005 per CPU-second
	MemoryCostPerGBSecond: 1, // 0.001 cents = $0.00001 per GB-second
}

// CalculateSessionCost calculates the total cost in cents for a session
func (c *CostCalculator) CalculateSessionCost(durationMs int64, cpuMillis int64, memoryMB int64) int64 {
	durationSeconds := float64(durationMs) / 1000.0
	cpuSeconds := float64(cpuMillis) / 1000.0
	memoryGBSeconds := float64(memoryMB) * durationSeconds / 1024.0

	baseCost := c.BaseCostPerRequest
	cpuCost := int64(cpuSeconds * float64(c.CPUCostPerSecond))
	memoryCost := int64(memoryGBSeconds * float64(c.MemoryCostPerGBSecond))

	return baseCost + cpuCost + memoryCost
}
