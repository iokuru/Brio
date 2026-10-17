module github.com/iokuru/brio/examples/refund-agent

go 1.22

require github.com/iokuru/brio v0.0.0

// Replace with local root module to reuse trace reader in tests
replace github.com/iokuru/brio => ../../
