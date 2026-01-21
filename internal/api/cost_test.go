package api

import "testing"

func TestCostCalculator_CalculateSessionCost(t *testing.T) {
	tests := []struct {
		name       string
		durationMs int64
		cpuMillis  int64
		memoryMB   int64
		wantMin    int64 // Minimum expected cost
		wantMax    int64 // Maximum expected cost
	}{
		{
			name:       "basic calculation with moderate usage",
			durationMs: 5000,  // 5 seconds
			cpuMillis:  5000,  // 5 CPU-seconds
			memoryMB:   512,   // 512MB
			wantMin:    1,     // At least base cost
			wantMax:    100,   // Reasonable upper bound
		},
		{
			name:       "zero duration returns base cost only",
			durationMs: 0,
			cpuMillis:  0,
			memoryMB:   0,
			wantMin:    1, // Base cost
			wantMax:    1, // Only base cost
		},
		{
			name:       "long running session",
			durationMs: 60000, // 1 minute
			cpuMillis:  60000, // 1 CPU-minute
			memoryMB:   1024,  // 1GB
			wantMin:    1,     // At least base cost
			wantMax:    500,   // Reasonable for 1 minute
		},
		{
			name:       "high CPU usage",
			durationMs: 10000,  // 10 seconds
			cpuMillis:  100000, // 100 CPU-seconds (10 cores)
			memoryMB:   256,
			wantMin:    100, // High CPU should cost more
			wantMax:    1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultCostCalculator.CalculateSessionCost(tt.durationMs, tt.cpuMillis, tt.memoryMB)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("CalculateSessionCost() = %d, want between %d and %d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCostCalculator_CustomRates(t *testing.T) {
	// Test with custom rates
	calculator := &CostCalculator{
		BaseCostPerRequest:    10, // 10 cents base
		CPUCostPerSecond:      1,  // 1 cent per CPU-second
		MemoryCostPerGBSecond: 1,  // 1 cent per GB-second
	}

	// 1 second, 1 CPU-second, 1GB
	cost := calculator.CalculateSessionCost(1000, 1000, 1024)
	// Expected: 10 (base) + 1 (1 CPU-second) + 1 (1 GB-second) = 12
	if cost < 10 || cost > 15 {
		t.Errorf("Custom calculator cost = %d, expected around 12", cost)
	}
}

func TestDefaultCostCalculator_Values(t *testing.T) {
	// Verify default calculator has reasonable values
	if DefaultCostCalculator.BaseCostPerRequest <= 0 {
		t.Error("BaseCostPerRequest should be positive")
	}
	if DefaultCostCalculator.CPUCostPerSecond <= 0 {
		t.Error("CPUCostPerSecond should be positive")
	}
	if DefaultCostCalculator.MemoryCostPerGBSecond <= 0 {
		t.Error("MemoryCostPerGBSecond should be positive")
	}
}
