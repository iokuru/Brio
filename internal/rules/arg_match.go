package rules

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/iokuru/brio/internal/trace"
)

type ArgMatchRule struct {
	Call      string
	Arg       string
	Predicate string
	Equals    any
	Regex     *regexp.Regexp
	RegexStr  string
	Present   bool
}

func NewArgMatch(call, arg, predicate string, equals any, regex *regexp.Regexp, regexStr string, present bool) *ArgMatchRule {
	return &ArgMatchRule{
		Call:      call,
		Arg:       arg,
		Predicate: predicate,
		Equals:    equals,
		Regex:     regex,
		RegexStr:  regexStr,
		Present:   present,
	}
}

func (r *ArgMatchRule) Name() string {
	return "arg_match"
}

func (r *ArgMatchRule) Check(t trace.Trace) Result {
	for _, call := range t.ToolCalls() {
		if call.Name != r.Call {
			continue
		}

		var val any
		hasArg := false
		if call.Args != nil {
			val, hasArg = call.Args[r.Arg]
		}

		switch r.Predicate {
		case "present":
			if r.Present && !hasArg {
				return Result{
					Pass:    false,
					Step:    call.Step,
					Message: fmt.Sprintf("%s missing required argument %q", r.Call, r.Arg),
				}
			}
			if !r.Present && hasArg {
				return Result{
					Pass:    false,
					Step:    call.Step,
					Message: fmt.Sprintf("%s unexpected argument %q present", r.Call, r.Arg),
				}
			}

		case "regex":
			if !hasArg {
				return Result{
					Pass:    false,
					Step:    call.Step,
					Message: fmt.Sprintf("%s missing argument %q", r.Call, r.Arg),
				}
			}
			strVal := fmt.Sprintf("%v", val)
			if !r.Regex.MatchString(strVal) {
				return Result{
					Pass:    false,
					Step:    call.Step,
					Message: fmt.Sprintf("%s argument %q with value %q did not match regex %q", r.Call, r.Arg, strVal, r.RegexStr),
				}
			}

		case "equals":
			if !hasArg {
				return Result{
					Pass:    false,
					Step:    call.Step,
					Message: fmt.Sprintf("%s missing argument %q", r.Call, r.Arg),
				}
			}
			if !compareValues(val, r.Equals) {
				return Result{
					Pass:    false,
					Step:    call.Step,
					Message: fmt.Sprintf("%s argument %q value %v does not equal %v", r.Call, r.Arg, val, r.Equals),
				}
			}
		}
	}

	return Result{Pass: true}
}

func compareValues(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}

	numA, okA := toFloat64(a)
	numB, okB := toFloat64(b)
	if okA && okB {
		return numA == numB
	}

	return false
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}
