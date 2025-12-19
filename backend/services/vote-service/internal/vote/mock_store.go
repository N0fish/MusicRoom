package vote

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockStore struct {
	mock.Mock
}

func (m *MockStore) LoadEvent(ctx context.Context, id string) (*Event, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Event), args.Error(1)
}

func (m *MockStore) IsInvited(ctx context.Context, eventID, userID string) (bool, error) {
	args := m.Called(ctx, eventID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockStore) CastVote(ctx context.Context, eventID, trackID, voterID string) error {
	args := m.Called(ctx, eventID, trackID, voterID)
	return args.Error(0)
}

func (m *MockStore) GetVoteCount(ctx context.Context, eventID, trackID string) (int, error) {
	args := m.Called(ctx, eventID, trackID)
	return args.Int(0), args.Error(1)
}

func (m *MockStore) GetVoteTally(ctx context.Context, eventID, voterID string) ([]Row, error) {
	args := m.Called(ctx, eventID, voterID)
	return args.Get(0).([]Row), args.Error(1)
}

func (m *MockStore) RemoveVote(ctx context.Context, eventID, trackID, voterID string) error {
	args := m.Called(ctx, eventID, trackID, voterID)
	return args.Error(0)
}

func (m *MockStore) ListEvents(ctx context.Context, userID, visibility string) ([]Event, error) {
	args := m.Called(ctx, userID, visibility)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Event), args.Error(1)
}

func (m *MockStore) CreateEvent(ctx context.Context, ev *Event) (string, error) {
	args := m.Called(ctx, ev)
	return args.String(0), args.Error(1)
}

func (m *MockStore) DeleteEvent(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStore) UpdateEvent(ctx context.Context, id string, updates map[string]any) error {
	args := m.Called(ctx, id, updates)
	return args.Error(0)
}

func (m *MockStore) TransferOwnership(ctx context.Context, id, newOwnerID string) error {
	args := m.Called(ctx, id, newOwnerID)
	return args.Error(0)
}

func (m *MockStore) GetParticipantRole(ctx context.Context, eventID, userID string) (string, error) {
	args := m.Called(ctx, eventID, userID)
	return args.String(0), args.Error(1)
}

func (m *MockStore) CreateInvite(ctx context.Context, eventID, userID, role string) error {
	args := m.Called(ctx, eventID, userID, role)
	return args.Error(0)
}

func (m *MockStore) DeleteInvite(ctx context.Context, eventID, userID string) error {
	args := m.Called(ctx, eventID, userID)
	return args.Error(0)
}

func (m *MockStore) ListInvites(ctx context.Context, eventID string) ([]Invite, error) {
	args := m.Called(ctx, eventID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Invite), args.Error(1)
}
