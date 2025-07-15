package consensus

import (
	"errors"
	"sort"
)

var (
	ErrNoActiveValidators = errors.New("no active validators")
)

type Manager struct {
	validators map[string]*Validator
}

func NewManager() *Manager {
	return &Manager{
		validators: make(map[string]*Validator),
	}
}

func (m *Manager) AddValidator(pubKey []byte, stake uint64) {
	m.validators[string(pubKey)] = &Validator{
		PublicKey: pubKey,
		Stake:     stake,
		IsActive:  true,
	}
}

func (m *Manager) GetValidator(pubKey []byte) (*Validator, bool) {
	v, ok := m.validators[string(pubKey)]
	return v, ok
}

func (m *Manager) SelectNextValidator() ([]byte, error) {
	var activeValidators []*Validator
	for _, v := range m.validators {
		if v.IsActive {
			activeValidators = append(activeValidators, v)
		}
	}

	if len(activeValidators) == 0 {
		return nil, ErrNoActiveValidators
	}

	sort.Slice(activeValidators, func(i, j int) bool {
		return activeValidators[i].LastBlockProduced < activeValidators[j].LastBlockProduced
	})

	return activeValidators[0].PublicKey, nil
}

func (m *Manager) RecordBlockProduction(pubKey []byte, blockIndex uint64) {
	if v, ok := m.GetValidator(pubKey); ok {
		v.LastBlockProduced = blockIndex
	}
}
