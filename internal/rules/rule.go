package rules

import "github.com/owner/brio/internal/trace"

type Result struct {
	Pass    bool
	Step    int
	Message string
}

type Rule interface {
	Name() string
	Check(t trace.Trace) Result
}
