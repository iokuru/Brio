package main

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed prompts/careful.txt
var carefulPrompt string

//go:embed prompts/naive.txt
var naivePrompt string

func GetPrompt(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "careful":
		return carefulPrompt, nil
	case "naive":
		return naivePrompt, nil
	default:
		return "", fmt.Errorf("unknown prompt variant %q (expected 'careful' or 'naive')", name)
	}
}
