package main

import (
	"errors"
	"testing"
)

func sampleTestStore() *Store {
	return NewStore([]User{
		{
			ID:        "u_1",
			Name:      "Alice",
			Status:    "active",
			Balance:   "100.00",
			Confirmed: false,
		},
		{
			ID:        "u_2",
			Name:      "Bob",
			Status:    "active",
			Balance:   "50.00",
			Confirmed: true,
		},
	})
}

func TestStoreLookup(t *testing.T) {
	s := sampleTestStore()

	tests := []struct {
		name     string
		id       string
		wantErr  error
		wantName string
	}{
		{
			name:     "existing user",
			id:       "u_1",
			wantErr:  nil,
			wantName: "Alice",
		},
		{
			name:     "unknown user",
			id:       "u_999",
			wantErr:  ErrUserNotFound,
			wantName: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := s.LookupUser(tc.id)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u.Name != tc.wantName {
				t.Errorf("expected name %q, got %q", tc.wantName, u.Name)
			}
		})
	}
}

func TestStoreConfirm(t *testing.T) {
	s := sampleTestStore()

	t.Run("confirm existing unconfirmed user", func(t *testing.T) {
		if err := s.ConfirmUser("u_1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		u, err := s.LookupUser("u_1")
		if err != nil {
			t.Fatalf("lookup failed: %v", err)
		}
		if !u.Confirmed {
			t.Errorf("expected user to be confirmed")
		}
	})

	t.Run("confirm unknown user", func(t *testing.T) {
		err := s.ConfirmUser("u_unknown")
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestStoreIssueRefund(t *testing.T) {
	s := sampleTestStore()

	tests := []struct {
		name    string
		id      string
		amount  string
		wantErr error
	}{
		{
			name:    "unconfirmed user fails",
			id:      "u_1",
			amount:  "20.00",
			wantErr: ErrUserNotConfirmed,
		},
		{
			name:    "confirmed user succeeds",
			id:      "u_2",
			amount:  "25.50",
			wantErr: nil,
		},
		{
			name:    "bad amount non-numeric",
			id:      "u_2",
			amount:  "invalid",
			wantErr: ErrInvalidAmount,
		},
		{
			name:    "bad amount negative",
			id:      "u_2",
			amount:  "-15.00",
			wantErr: ErrInvalidAmount,
		},
		{
			name:    "bad amount too many decimals",
			id:      "u_2",
			amount:  "15.999",
			wantErr: ErrInvalidAmount,
		},
		{
			name:    "unknown user fails",
			id:      "u_none",
			amount:  "10.00",
			wantErr: ErrUserNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := s.IssueRefund(tc.id, tc.amount)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestStoreDeleteUser(t *testing.T) {
	s := sampleTestStore()

	t.Run("delete existing user", func(t *testing.T) {
		if err := s.DeleteUser("u_1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err := s.LookupUser("u_1")
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound after deletion, got %v", err)
		}
	})

	t.Run("delete unknown user", func(t *testing.T) {
		err := s.DeleteUser("u_none")
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestExecuteTool(t *testing.T) {
	s := sampleTestStore()

	t.Run("lookup_user tool", func(t *testing.T) {
		res, err := ExecuteTool(s, "lookup_user", map[string]any{"id": "u_1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res["name"] != "Alice" {
			t.Errorf("expected Alice, got %v", res["name"])
		}
	})

	t.Run("confirm_user tool", func(t *testing.T) {
		res, err := ExecuteTool(s, "confirm_user", map[string]any{"id": "u_1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res["ok"] != true {
			t.Errorf("expected ok=true, got %v", res["ok"])
		}
	})

	t.Run("issue_refund tool", func(t *testing.T) {
		res, err := ExecuteTool(s, "issue_refund", map[string]any{"id": "u_2", "amount": "10.00"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res["ok"] != true || res["refund"] != "10.00" {
			t.Errorf("unexpected refund result: %v", res)
		}
	})

	t.Run("issue_refund unconfirmed", func(t *testing.T) {
		freshStore := sampleTestStore()
		_, err := ExecuteTool(freshStore, "issue_refund", map[string]any{"id": "u_1", "amount": "10.00"})
		if !errors.Is(err, ErrUserNotConfirmed) {
			t.Fatalf("expected ErrUserNotConfirmed, got %v", err)
		}
	})

	t.Run("delete_user tool", func(t *testing.T) {
		res, err := ExecuteTool(s, "delete_user", map[string]any{"id": "u_1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res["ok"] != true {
			t.Errorf("expected ok=true, got %v", res["ok"])
		}
	})

	t.Run("unknown tool error", func(t *testing.T) {
		_, err := ExecuteTool(s, "hack_system", map[string]any{"id": "u_1"})
		if err == nil {
			t.Fatal("expected error for unknown tool")
		}
	})

	t.Run("missing arguments", func(t *testing.T) {
		_, err := ExecuteTool(s, "lookup_user", map[string]any{})
		if err == nil {
			t.Fatal("expected error for missing id")
		}
	})
}
