package rules

import "github.com/iokuru/brio/internal/trace"

type Result struct {
	Pass    bool
	Step    int
	Message string
}

type Rule interface {
	Name() string
	Check(t trace.Trace) Result
}
