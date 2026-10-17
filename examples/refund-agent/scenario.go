package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Scenario struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	UserMessage string `json:"user_message"`
	Spec        string `json:"spec"`
	Users       []User `json:"users"`
}

func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scenario %s: %w", path, err)
	}
	var sc Scenario
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("unmarshal scenario %s: %w", path, err)
	}
	return &sc, nil
}

func LoadAllScenarios(dir string) ([]*Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read scenarios dir %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)

	var scenarios []*Scenario
	for _, f := range files {
		sc, err := LoadScenario(f)
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, sc)
	}
	return scenarios, nil
}

func FindScenario(scenarios []*Scenario, id string) (*Scenario, error) {
	id = strings.TrimSpace(id)
	for _, sc := range scenarios {
		if sc.ID == id || strings.TrimPrefix(sc.ID, "0") == strings.TrimPrefix(id, "0") {
			return sc, nil
		}
	}
	return nil, fmt.Errorf("scenario %q not found", id)
}
