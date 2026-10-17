package rules

import (
	"fmt"

	"github.com/iokuru/brio/internal/trace"
)

type OrderPair struct {
	Before      string
	After       string
	Immediately bool
}

type OrderRule struct {
	Pairs []OrderPair
}

func NewOrder(pairs []OrderPair) *OrderRule {
	return &OrderRule{Pairs: pairs}
}

func (r *OrderRule) Name() string {
	return "order"
}

func (r *OrderRule) Check(t trace.Trace) Result {
	calls := t.ToolCalls()

	for _, pair := range r.Pairs {
		var beforeIndices []int
		var afterIndices []int

		for idx, call := range calls {
			if call.Name == pair.Before {
				beforeIndices = append(beforeIndices, idx)
			}
			if call.Name == pair.After {
				afterIndices = append(afterIndices, idx)
			}
		}

		if len(afterIndices) > 0 && len(beforeIndices) == 0 {
			firstAfter := calls[afterIndices[0]]
			return Result{
				Pass:    false,
				Step:    firstAfter.Step,
				Message: fmt.Sprintf("%s called before %s", pair.After, pair.Before),
			}
		}

		if len(beforeIndices) > 0 && len(afterIndices) > 0 {
			firstAfterIdx := afterIndices[0]
			firstBeforeIdx := beforeIndices[0]

			if firstAfterIdx < firstBeforeIdx {
				offending := calls[firstAfterIdx]
				return Result{
					Pass:    false,
					Step:    offending.Step,
					Message: fmt.Sprintf("%s called before %s", pair.After, pair.Before),
				}
			}

			for _, bIdx := range beforeIndices {
				if bIdx > firstAfterIdx {
					offending := calls[bIdx]
					return Result{
						Pass:    false,
						Step:    offending.Step,
						Message: fmt.Sprintf("%s called after %s", pair.Before, pair.After),
					}
				}
			}
		}

		if pair.Immediately {
			for _, bIdx := range beforeIndices {
				nextIdx := bIdx + 1
				if nextIdx >= len(calls) {
					offending := calls[bIdx]
					return Result{
						Pass:    false,
						Step:    offending.Step,
						Message: fmt.Sprintf("%s not immediately followed by %s", pair.Before, pair.After),
					}
				}

				nextCall := calls[nextIdx]
				if nextCall.Name != pair.After {
					return Result{
						Pass:    false,
						Step:    nextCall.Step,
						Message: fmt.Sprintf("%s called after %s, expected %s", nextCall.Name, pair.Before, pair.After),
					}
				}
			}
		}
	}

	return Result{Pass: true}
}
