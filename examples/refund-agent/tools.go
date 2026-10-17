package main

import (
	"fmt"
)

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

var AgentTools = []Tool{
	{
		Name:        "lookup_user",
		Description: "Look up a user's details including name, status, balance, and confirmation status by user ID.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "The unique user ID (e.g. u_42).",
				},
			},
			"required": []string{"id"},
		},
	},
	{
		Name:        "confirm_user",
		Description: "Confirm and verify a user's identity before performing account actions.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "The unique user ID to confirm.",
				},
			},
			"required": []string{"id"},
		},
	},
	{
		Name:        "issue_refund",
		Description: "Issue a monetary refund to a confirmed user. Fails if user is not verified.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "The unique user ID.",
				},
				"amount": map[string]any{
					"type":        "string",
					"description": "The refund amount with two decimals, e.g. '49.99'.",
				},
			},
			"required": []string{"id", "amount"},
		},
	},
	{
		Name:        "delete_user",
		Description: "Permanently delete a customer account. Destructive action.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "The unique user ID to delete.",
				},
			},
			"required": []string{"id"},
		},
	},
}

func ExecuteTool(store *Store, name string, args map[string]any) (map[string]any, error) {
	switch name {
	case "lookup_user":
		id, _ := args["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("missing or invalid id argument")
		}
		u, err := store.LookupUser(id)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"id":        u.ID,
			"name":      u.Name,
			"status":    u.Status,
			"balance":   u.Balance,
			"confirmed": u.Confirmed,
		}, nil

	case "confirm_user":
		id, _ := args["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("missing or invalid id argument")
		}
		if err := store.ConfirmUser(id); err != nil {
			return nil, err
		}
		return map[string]any{
			"ok":      true,
			"message": fmt.Sprintf("user %s confirmed", id),
		}, nil

	case "issue_refund":
		id, _ := args["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("missing or invalid id argument")
		}
		amount, _ := args["amount"].(string)
		if amount == "" {
			if num, ok := args["amount"].(float64); ok {
				amount = fmt.Sprintf("%.2f", num)
			} else {
				return nil, fmt.Errorf("missing or invalid amount argument")
			}
		}
		if err := store.IssueRefund(id, amount); err != nil {
			return nil, err
		}
		return map[string]any{
			"ok":      true,
			"refund":  amount,
			"message": fmt.Sprintf("refund of %s issued to user %s", amount, id),
		}, nil

	case "delete_user":
		id, _ := args["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("missing or invalid id argument")
		}
		if err := store.DeleteUser(id); err != nil {
			return nil, err
		}
		return map[string]any{
			"ok":      true,
			"message": fmt.Sprintf("user %s deleted", id),
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
