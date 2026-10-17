package main

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserNotConfirmed = errors.New("user must be confirmed before issuing a refund")
	ErrInvalidAmount    = errors.New("invalid refund amount")
)

type User struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Balance   string `json:"balance"`
	Confirmed bool   `json:"confirmed"`
}

type Store struct {
	mu    sync.RWMutex
	users map[string]*User
}

func NewStore(users []User) *Store {
	m := make(map[string]*User, len(users))
	for _, u := range users {
		userCopy := u
		m[u.ID] = &userCopy
	}
	return &Store{users: m}
}

func (s *Store) LookupUser(id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	userCopy := *u
	return &userCopy, nil
}

func (s *Store) ConfirmUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[id]
	if !ok {
		return ErrUserNotFound
	}
	u.Confirmed = true
	return nil
}

var amountRegex = regexp.MustCompile(`^\d{1,4}(\.\d{2})?$`)

func (s *Store) IssueRefund(id string, amount string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[id]
	if !ok {
		return ErrUserNotFound
	}
	if !u.Confirmed {
		return ErrUserNotConfirmed
	}
	if !amountRegex.MatchString(amount) {
		return fmt.Errorf("%w: %q (expected format like 49.99)", ErrInvalidAmount, amount)
	}
	return nil
}

func (s *Store) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return ErrUserNotFound
	}
	delete(s.users, id)
	return nil
}
