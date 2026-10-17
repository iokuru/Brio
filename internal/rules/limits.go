package rules

import (
	"fmt"

	"github.com/owner/brio/internal/trace"
)

type LimitsRule struct {
	MaxSteps       *int
	MaxCostUSD     *float64
	MaxLatencyMS   *int64
	MaxRepeatCalls *int
}

func NewLimits(maxSteps *int, maxCost *float64, maxLatency *int64, maxRepeat *int) *LimitsRule {
	return &LimitsRule{
		MaxSteps:       maxSteps,
		MaxCostUSD:     maxCost,
		MaxLatencyMS:   maxLatency,
		MaxRepeatCalls: maxRepeat,
	}
}

func (r *LimitsRule) Name() string {
	return "limits"
}

func (r *LimitsRule) Check(t trace.Trace) Result {
	stepCount := 0
	totalCost := 0.0
	totalLatency := int64(0)
	repeatCounts := make(map[string]int)

	for _, s := range t.Steps {
		stepCount++
		totalCost += s.CostUSD
		totalLatency += s.LatencyMS

		if s.Type == "tool_call" && r.MaxRepeatCalls != nil {
			repeatCounts[s.Name]++
			if repeatCounts[s.Name] > *r.MaxRepeatCalls {
				return Result{
					Pass:    false,
					Step:    s.Step,
					Message: fmt.Sprintf("%s called %d times (max_repeat_calls: %d)", s.Name, repeatCounts[s.Name], *r.MaxRepeatCalls),
				}
			}
		}

		if r.MaxSteps != nil && stepCount > *r.MaxSteps {
			return Result{
				Pass:    false,
				Step:    s.Step,
				Message: fmt.Sprintf("exceeded max_steps: %d (max_steps: %d)", stepCount, *r.MaxSteps),
			}
		}

		if r.MaxCostUSD != nil && totalCost > *r.MaxCostUSD+1e-9 {
			return Result{
				Pass:    false,
				Step:    s.Step,
				Message: fmt.Sprintf("exceeded max_cost_usd: %.4f (max_cost_usd: %.4f)", totalCost, *r.MaxCostUSD),
			}
		}

		if r.MaxLatencyMS != nil && totalLatency > *r.MaxLatencyMS {
			return Result{
				Pass:    false,
				Step:    s.Step,
				Message: fmt.Sprintf("exceeded max_latency_ms: %d (max_latency_ms: %d)", totalLatency, *r.MaxLatencyMS),
			}
		}
	}

	return Result{Pass: true}
}
