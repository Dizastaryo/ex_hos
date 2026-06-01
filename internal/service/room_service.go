package service

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/internal/ws"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type RoomService struct {
	repo   *postgres.RoomRepository
	hub    *ws.Hub
	logger *zap.Logger
}

func NewRoomService(repo *postgres.RoomRepository, hub *ws.Hub, logger *zap.Logger) *RoomService {
	return &RoomService{repo: repo, hub: hub, logger: logger}
}

func (s *RoomService) Create(ctx context.Context, creatorID string, req *domain.CreateRoomRequest) (*domain.Room, error) {
	// All rooms are voice+text and private — regardless of what the client sends.
	req.Type = "voice"
	req.IsPublic = false
	room, err := s.repo.Create(ctx, req, creatorID)
	if err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return room, nil
}

func (s *RoomService) GetByID(ctx context.Context, id, viewerID string) (*domain.Room, error) {
	return s.repo.GetByID(ctx, id, viewerID)
}

func (s *RoomService) List(ctx context.Context, viewerID string, page, limit int) ([]*domain.Room, pagination.Meta, error) {
	offset := pagination.Offset(page, limit)
	items, err := s.repo.List(ctx, viewerID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, err
	}
	hasNext := len(items) > limit
	if hasNext {
		items = items[:limit]
	}
	return items, pagination.Meta{Page: page, Limit: limit, HasNextPage: hasNext}, nil
}

func (s *RoomService) Join(ctx context.Context, roomID, userID string) (*domain.Room, error) {
	// repo.Join already checks is_public; private rooms return ErrPrivateRoom.
	if err := s.repo.Join(ctx, roomID, userID); err != nil {
		return nil, err
	}
	room, err := s.repo.GetByID(ctx, roomID, userID)
	if err != nil {
		return nil, err
	}
	if !room.IsActive {
		return nil, domain.ErrRoomClosed
	}
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	s.hub.SendToUsers(ids, "room.joined", map[string]any{
		"room_id":  roomID,
		"user_id":  userID,
		"is_muted": false,
	})
	return room, nil
}

func (s *RoomService) InviteMember(ctx context.Context, roomID, inviterID, userID string) error {
	if err := s.repo.InviteMember(ctx, roomID, inviterID, userID); err != nil {
		return err
	}
	// Notify the invited user so their room list refreshes.
	s.hub.SendToUsers([]string{userID}, "room.invited", map[string]any{
		"room_id": roomID,
	})
	// Notify existing members that someone new was added.
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	s.hub.SendToUsers(ids, "room.member_added", map[string]any{
		"room_id": roomID,
		"user_id": userID,
	})
	return nil
}

func (s *RoomService) RemoveMember(ctx context.Context, roomID, requesterID, targetID string) error {
	if err := s.repo.RemoveMember(ctx, roomID, requesterID, targetID); err != nil {
		return err
	}
	// Notify the removed user.
	s.hub.SendToUsers([]string{targetID}, "room.removed", map[string]any{
		"room_id": roomID,
	})
	// Notify remaining members.
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	s.hub.SendToUsers(ids, "room.member_removed", map[string]any{
		"room_id": roomID,
		"user_id": targetID,
	})
	return nil
}

func (s *RoomService) GetMembers(ctx context.Context, roomID, viewerID string) ([]domain.RoomMember, error) {
	return s.repo.ListMembers(ctx, roomID, viewerID)
}

func (s *RoomService) Leave(ctx context.Context, roomID, userID string) error {
	// Remove from voice channel first (silently, user may not be in voice)
	s.repo.LeaveVoiceIfPresent(ctx, roomID, userID)
	if err := s.repo.Leave(ctx, roomID, userID); err != nil {
		return err // preserve sentinel errors (ErrForbidden, ErrRoomNotFound)
	}
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	ids = append(ids, userID)
	s.hub.SendToUsers(ids, "room.left", map[string]any{
		"room_id": roomID,
		"user_id": userID,
	})
	return nil
}

// JoinVoice puts the user into the voice channel of the room they are already a member of.
// Broadcasts room.voice.joined to all room participants.
func (s *RoomService) JoinVoice(ctx context.Context, roomID, userID string) error {
	if err := s.repo.JoinVoice(ctx, roomID, userID); err != nil {
		return err
	}
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	s.hub.SendToUsers(ids, "room.voice.joined", map[string]any{
		"room_id": roomID,
		"user_id": userID,
	})
	return nil
}

// LeaveVoice removes the user from the voice channel without removing them from the room.
// Broadcasts room.voice.left to all room participants.
func (s *RoomService) LeaveVoice(ctx context.Context, roomID, userID string) error {
	if err := s.repo.LeaveVoice(ctx, roomID, userID); err != nil {
		return err
	}
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	s.hub.SendToUsers(ids, "room.voice.left", map[string]any{
		"room_id": roomID,
		"user_id": userID,
	})
	return nil
}

func (s *RoomService) ToggleMute(ctx context.Context, roomID, userID string) (bool, error) {
	muted, err := s.repo.IsMuted(ctx, roomID, userID)
	if err != nil {
		return false, err
	}
	newMuted := !muted
	if err := s.repo.SetMuted(ctx, roomID, userID, newMuted); err != nil {
		return false, fmt.Errorf("set muted: %w", err)
	}
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	s.hub.SendToUsers(ids, "room.muted", map[string]any{
		"room_id":  roomID,
		"user_id":  userID,
		"is_muted": newMuted,
	})
	return newMuted, nil
}

func (s *RoomService) Update(ctx context.Context, roomID, requesterID string, req *domain.UpdateRoomRequest) (*domain.Room, error) {
	if err := s.repo.UpdateRoom(ctx, roomID, requesterID, req); err != nil {
		return nil, err
	}
	room, err := s.repo.GetByID(ctx, roomID, requesterID)
	if err != nil {
		return nil, err
	}
	// Notify all members of the update
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	s.hub.SendToUsers(ids, "room.updated", map[string]any{
		"room_id":     roomID,
		"name":        room.Name,
		"description": room.Description,
		"cover_url":   room.CoverURL,
	})
	return room, nil
}

func (s *RoomService) SetAdmin(ctx context.Context, roomID, requesterID, targetID string, grant bool) error {
	if err := s.repo.SetAdmin(ctx, roomID, requesterID, targetID, grant); err != nil {
		return err
	}
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	s.hub.SendToUsers(ids, "room.admin_changed", map[string]any{
		"room_id":  roomID,
		"user_id":  targetID,
		"is_admin": grant,
	})
	return nil
}

func (s *RoomService) Close(ctx context.Context, roomID, userID string) error {
	room, err := s.repo.GetByID(ctx, roomID, userID)
	if err != nil {
		return err
	}
	if room.CreatorID != userID {
		return domain.ErrForbidden
	}
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	if err := s.repo.Close(ctx, roomID); err != nil {
		return fmt.Errorf("close room: %w", err)
	}
	s.hub.SendToUsers(ids, "room.closed", map[string]any{
		"room_id": roomID,
	})
	return nil
}

func (s *RoomService) SendMessage(ctx context.Context, roomID, senderID, text string) (*domain.RoomMessage, error) {
	room, err := s.repo.GetByID(ctx, roomID, senderID)
	if err != nil {
		return nil, err
	}
	if !room.IsActive {
		return nil, domain.ErrRoomClosed
	}
	if !room.IsJoined {
		return nil, domain.ErrForbidden
	}
	msg, err := s.repo.SendMessage(ctx, roomID, senderID, text)
	if err != nil {
		return nil, err
	}
	ids, _ := s.repo.GetParticipantIDs(ctx, roomID)
	s.hub.SendToUsers(ids, "room.message", map[string]any{
		"room_id": roomID,
		"message": msg,
	})
	return msg, nil
}

func (s *RoomService) GetMessages(ctx context.Context, roomID, viewerID string, page, limit int) ([]*domain.RoomMessage, pagination.Meta, error) {
	// Only members may read messages.
	if _, err := s.repo.IsMuted(ctx, roomID, viewerID); err != nil {
		if err == domain.ErrNotInRoom {
			return nil, pagination.Meta{}, domain.ErrForbidden
		}
		return nil, pagination.Meta{}, err
	}
	offset := pagination.Offset(page, limit)
	items, err := s.repo.GetMessages(ctx, roomID, limit+1, offset)
	if err != nil {
		return nil, pagination.Meta{}, err
	}
	hasNext := len(items) > limit
	if hasNext {
		items = items[:limit]
	}
	return items, pagination.Meta{Page: page, Limit: limit, HasNextPage: hasNext}, nil
}
