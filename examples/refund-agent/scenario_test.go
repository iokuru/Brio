package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrompts(t *testing.T) {
	careful, err := GetPrompt("careful")
	if err != nil {
		t.Fatalf("unexpected error for careful prompt: %v", err)
	}
	if len(careful) == 0 {
		t.Fatal("expected non-empty careful prompt")
	}

	naive, err := GetPrompt("naive")
	if err != nil {
		t.Fatalf("unexpected error for naive prompt: %v", err)
	}
	if len(naive) == 0 {
		t.Fatal("expected non-empty naive prompt")
	}

	_, err = GetPrompt("invalid")
	if err == nil {
		t.Fatal("expected error for invalid prompt name")
	}
}

func TestScenarios(t *testing.T) {
	scenariosDir := filepath.Join(".", "scenarios")
	scenarios, err := LoadAllScenarios(scenariosDir)
	if err != nil {
		t.Fatalf("failed to load scenarios: %v", err)
	}

	if len(scenarios) != 8 {
		t.Fatalf("expected 8 scenarios, got %d", len(scenarios))
	}

	for _, sc := range scenarios {
		if sc.ID == "" {
			t.Errorf("scenario missing ID: %+v", sc)
		}
		if sc.Title == "" {
			t.Errorf("scenario %s missing title", sc.ID)
		}
		if sc.UserMessage == "" {
			t.Errorf("scenario %s missing user message", sc.ID)
		}
		if sc.Spec == "" {
			t.Errorf("scenario %s missing spec", sc.ID)
		}
		if len(sc.Users) == 0 {
			t.Errorf("scenario %s missing seeded store users", sc.ID)
		}
	}

	sc1, err := FindScenario(scenarios, "1")
	if err != nil || sc1 == nil || sc1.ID != "1" {
		t.Errorf("failed to find scenario 1: %v", err)
	}

	sc2, err := FindScenario(scenarios, "02")
	if err != nil || sc2 == nil || sc2.ID != "2" {
		t.Errorf("failed to find scenario 02: %v", err)
	}

	_, err = FindScenario(scenarios, "999")
	if err == nil {
		t.Errorf("expected error for non-existent scenario")
	}
}

func TestSpecsExist(t *testing.T) {
	for _, specFile := range []string{"brio.yaml", "brio.not-found.yaml"} {
		info, err := os.Stat(specFile)
		if err != nil {
			t.Fatalf("spec file %s not found: %v", specFile, err)
		}
		if info.Size() == 0 {
			t.Errorf("spec file %s is empty", specFile)
		}
	}
}
