package spec

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/owner/brio/internal/rules"
	"gopkg.in/yaml.v3"
)

type ValidationError struct {
	File   string
	Index  int
	Reason string
}

func (e *ValidationError) Error() string {
	if e.Index >= 0 {
		return fmt.Sprintf("%s: rule %d: %s", e.File, e.Index, e.Reason)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Reason)
}

type OrderItem struct {
	Before      string
	After       string
	Immediately bool
}

type ArgMatchDef struct {
	Call      string
	Arg       string
	Predicate string
	Equals    any
	Regex     *regexp.Regexp
	RegexStr  string
	Present   bool
}

type LimitsDef struct {
	MaxSteps       *int
	MaxCostUSD     *float64
	MaxLatencyMS   *int64
	MaxRepeatCalls *int
}

type RuleDef struct {
	Index          int
	Kind           string
	RequiredCalls  []string
	ForbiddenCalls []string
	Order          []OrderItem
	Subsequence    []string
	ArgMatch       *ArgMatchDef
	Limits         *LimitsDef
}

type Spec struct {
	Name  string
	Path  string
	Rules []RuleDef
}

func (s *Spec) BuildRules() []rules.Rule {
	var result []rules.Rule
	for _, r := range s.Rules {
		switch r.Kind {
		case "required_calls":
			result = append(result, rules.NewRequiredCalls(r.RequiredCalls))
		case "forbidden_calls":
			result = append(result, rules.NewForbiddenCalls(r.ForbiddenCalls))
		case "order":
			var pairs []rules.OrderPair
			for _, item := range r.Order {
				pairs = append(pairs, rules.OrderPair{
					Before:      item.Before,
					After:       item.After,
					Immediately: item.Immediately,
				})
			}
			result = append(result, rules.NewOrder(pairs))
		case "subsequence":
			result = append(result, rules.NewSubsequence(r.Subsequence))
		case "arg_match":
			result = append(result, rules.NewArgMatch(
				r.ArgMatch.Call,
				r.ArgMatch.Arg,
				r.ArgMatch.Predicate,
				r.ArgMatch.Equals,
				r.ArgMatch.Regex,
				r.ArgMatch.RegexStr,
				r.ArgMatch.Present,
			))
		case "limits":
			result = append(result, rules.NewLimits(
				r.Limits.MaxSteps,
				r.Limits.MaxCostUSD,
				r.Limits.MaxLatencyMS,
				r.Limits.MaxRepeatCalls,
			))
		}
	}
	return result
}

func Load(r io.Reader, path string) (*Spec, error) {
	var doc yaml.Node
	if err := yaml.NewDecoder(r).Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &ValidationError{File: path, Index: -1, Reason: "spec file is empty"}
		}
		return nil, fmt.Errorf("%s: malformed YAML: %w", path, err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, &ValidationError{File: path, Index: -1, Reason: "spec file is empty"}
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, &ValidationError{File: path, Index: -1, Reason: "spec root must be a mapping"}
	}

	sp := &Spec{Path: path}
	var rulesNode *yaml.Node

	for i := 0; i < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]

		switch keyNode.Value {
		case "name":
			sp.Name = strings.TrimSpace(valNode.Value)
		case "rules":
			rulesNode = valNode
		default:
			return nil, &ValidationError{
				File:   path,
				Index:  -1,
				Reason: fmt.Sprintf("unknown spec key %q", keyNode.Value),
			}
		}
	}

	if sp.Name == "" {
		return nil, &ValidationError{File: path, Index: -1, Reason: "spec name is required"}
	}

	if rulesNode == nil {
		return nil, &ValidationError{File: path, Index: -1, Reason: "spec rules list is required"}
	}

	if rulesNode.Kind != yaml.SequenceNode || len(rulesNode.Content) == 0 {
		return nil, &ValidationError{File: path, Index: -1, Reason: "rules must be a non-empty list"}
	}

	for idx, ruleNode := range rulesNode.Content {
		ruleDef, err := parseRuleNode(ruleNode, path, idx)
		if err != nil {
			return nil, err
		}
		sp.Rules = append(sp.Rules, ruleDef)
	}

	return sp, nil
}

func LoadFile(path string) (*Spec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spec file: %w", err)
	}
	defer f.Close()

	return Load(f, path)
}

func parseRuleNode(node *yaml.Node, file string, index int) (RuleDef, error) {
	if node.Kind != yaml.MappingNode {
		return RuleDef{}, &ValidationError{
			File:   file,
			Index:  index,
			Reason: "rule must be a mapping with a single rule key",
		}
	}

	if len(node.Content) == 0 {
		return RuleDef{}, &ValidationError{
			File:   file,
			Index:  index,
			Reason: "empty rule definition",
		}
	}

	if len(node.Content) > 2 {
		return RuleDef{}, &ValidationError{
			File:   file,
			Index:  index,
			Reason: "rule must contain exactly one rule key",
		}
	}

	key := node.Content[0].Value
	val := node.Content[1]
	rd := RuleDef{
		Index: index,
		Kind:  key,
	}

	switch key {
	case "required_calls":
		calls, err := parseStringList(val, "required_calls")
		if err != nil {
			return RuleDef{}, &ValidationError{File: file, Index: index, Reason: err.Error()}
		}
		rd.RequiredCalls = calls

	case "forbidden_calls":
		calls, err := parseStringList(val, "forbidden_calls")
		if err != nil {
			return RuleDef{}, &ValidationError{File: file, Index: index, Reason: err.Error()}
		}
		rd.ForbiddenCalls = calls

	case "order":
		orders, err := parseOrderList(val)
		if err != nil {
			return RuleDef{}, &ValidationError{File: file, Index: index, Reason: err.Error()}
		}
		rd.Order = orders

	case "subsequence":
		calls, err := parseStringList(val, "subsequence")
		if err != nil {
			return RuleDef{}, &ValidationError{File: file, Index: index, Reason: err.Error()}
		}
		rd.Subsequence = calls

	case "arg_match":
		am, err := parseArgMatch(val)
		if err != nil {
			return RuleDef{}, &ValidationError{File: file, Index: index, Reason: err.Error()}
		}
		rd.ArgMatch = am

	case "limits":
		lims, err := parseLimits(val)
		if err != nil {
			return RuleDef{}, &ValidationError{File: file, Index: index, Reason: err.Error()}
		}
		rd.Limits = lims

	default:
		return RuleDef{}, &ValidationError{
			File:   file,
			Index:  index,
			Reason: fmt.Sprintf("unknown rule key %q", key),
		}
	}

	return rd, nil
}

func parseStringList(node *yaml.Node, ruleName string) ([]string, error) {
	if node.Kind != yaml.SequenceNode || len(node.Content) == 0 {
		return nil, fmt.Errorf("%s list cannot be empty", ruleName)
	}

	var res []string
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || strings.TrimSpace(item.Value) == "" {
			return nil, fmt.Errorf("%s items must be non-empty strings", ruleName)
		}
		res = append(res, strings.TrimSpace(item.Value))
	}
	return res, nil
}

func parseOrderList(node *yaml.Node) ([]OrderItem, error) {
	if node.Kind != yaml.SequenceNode || len(node.Content) == 0 {
		return nil, fmt.Errorf("order list cannot be empty")
	}

	var res []OrderItem
	for _, itemNode := range node.Content {
		if itemNode.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("order item must be a mapping")
		}

		var item OrderItem
		var hasBefore, hasAfter bool

		for i := 0; i < len(itemNode.Content); i += 2 {
			k := itemNode.Content[i].Value
			v := itemNode.Content[i+1]

			switch k {
			case "before":
				if strings.TrimSpace(v.Value) == "" {
					return nil, fmt.Errorf("order before cannot be empty")
				}
				item.Before = strings.TrimSpace(v.Value)
				hasBefore = true
			case "after":
				if strings.TrimSpace(v.Value) == "" {
					return nil, fmt.Errorf("order after cannot be empty")
				}
				item.After = strings.TrimSpace(v.Value)
				hasAfter = true
			case "immediately":
				var b bool
				if err := v.Decode(&b); err != nil {
					return nil, fmt.Errorf("order immediately must be a boolean: %w", err)
				}
				item.Immediately = b
			default:
				return nil, fmt.Errorf("order unknown key %q", k)
			}
		}

		if !hasBefore {
			return nil, fmt.Errorf("order before is required")
		}
		if !hasAfter {
			return nil, fmt.Errorf("order after is required")
		}
		res = append(res, item)
	}

	return res, nil
}

func parseArgMatch(node *yaml.Node) (*ArgMatchDef, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("arg_match must be a mapping")
	}

	def := &ArgMatchDef{}
	var hasCall, hasArg bool
	predicateCount := 0

	for i := 0; i < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]

		switch k {
		case "call":
			if strings.TrimSpace(v.Value) == "" {
				return nil, fmt.Errorf("arg_match call cannot be empty")
			}
			def.Call = strings.TrimSpace(v.Value)
			hasCall = true
		case "arg":
			if strings.TrimSpace(v.Value) == "" {
				return nil, fmt.Errorf("arg_match arg cannot be empty")
			}
			def.Arg = strings.TrimSpace(v.Value)
			hasArg = true
		case "equals":
			var val any
			if err := v.Decode(&val); err != nil {
				return nil, fmt.Errorf("arg_match equals decode error: %w", err)
			}
			def.Predicate = "equals"
			def.Equals = val
			predicateCount++
		case "regex":
			var s string
			if err := v.Decode(&s); err != nil {
				return nil, fmt.Errorf("arg_match regex must be string: %w", err)
			}
			re, err := regexp.Compile(s)
			if err != nil {
				return nil, fmt.Errorf("arg_match invalid regex %q: %w", s, err)
			}
			def.Predicate = "regex"
			def.Regex = re
			def.RegexStr = s
			predicateCount++
		case "present":
			var b bool
			if err := v.Decode(&b); err != nil {
				return nil, fmt.Errorf("arg_match present must be boolean: %w", err)
			}
			def.Predicate = "present"
			def.Present = b
			predicateCount++
		default:
			return nil, fmt.Errorf("arg_match unknown key %q", k)
		}
	}

	if !hasCall {
		return nil, fmt.Errorf("arg_match call is required")
	}
	if !hasArg {
		return nil, fmt.Errorf("arg_match arg is required")
	}
	if predicateCount != 1 {
		return nil, fmt.Errorf("arg_match must specify exactly one predicate (equals, regex, present)")
	}

	return def, nil
}

func parseLimits(node *yaml.Node) (*LimitsDef, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("limits must be a mapping")
	}

	lims := &LimitsDef{}
	hasAny := false

	for i := 0; i < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]

		switch k {
		case "max_steps":
			var val int
			if err := v.Decode(&val); err != nil {
				return nil, fmt.Errorf("limits max_steps must be integer: %w", err)
			}
			if val < 0 {
				return nil, fmt.Errorf("limits max_steps cannot be negative: %d", val)
			}
			lims.MaxSteps = &val
			hasAny = true
		case "max_cost_usd":
			var val float64
			if err := v.Decode(&val); err != nil {
				return nil, fmt.Errorf("limits max_cost_usd must be number: %w", err)
			}
			if val < 0 {
				return nil, fmt.Errorf("limits max_cost_usd cannot be negative: %v", val)
			}
			lims.MaxCostUSD = &val
			hasAny = true
		case "max_latency_ms":
			var val int64
			if err := v.Decode(&val); err != nil {
				return nil, fmt.Errorf("limits max_latency_ms must be integer: %w", err)
			}
			if val < 0 {
				return nil, fmt.Errorf("limits max_latency_ms cannot be negative: %d", val)
			}
			lims.MaxLatencyMS = &val
			hasAny = true
		case "max_repeat_calls":
			var val int
			if err := v.Decode(&val); err != nil {
				return nil, fmt.Errorf("limits max_repeat_calls must be integer: %w", err)
			}
			if val < 0 {
				return nil, fmt.Errorf("limits max_repeat_calls cannot be negative: %d", val)
			}
			lims.MaxRepeatCalls = &val
			hasAny = true
		default:
			return nil, fmt.Errorf("limits unknown key %q", k)
		}
	}

	if !hasAny {
		return nil, fmt.Errorf("limits must specify at least one limit")
	}

	return lims, nil
}
