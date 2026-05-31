package service

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"github.com/seeu/backend/pkg/pagination"
	"go.uber.org/zap"
)

type SearchResult struct {
	Users []*domain.UserShort `json:"users,omitempty"`
	Posts []*domain.Post      `json:"posts,omitempty"`
}

type SearchService struct {
	userRepo *postgres.UserRepository
	postRepo *postgres.PostRepository
	logger   *zap.Logger
}

func NewSearchService(
	userRepo *postgres.UserRepository,
	postRepo *postgres.PostRepository,
	logger *zap.Logger,
) *SearchService {
	return &SearchService{
		userRepo: userRepo,
		postRepo: postRepo,
		logger:   logger,
	}
}

func (s *SearchService) Search(ctx context.Context, query, searchType string, page, limit int) (*SearchResult, pagination.Meta, error) {
	if query == "" {
		return &SearchResult{}, pagination.Meta{Page: page, Limit: limit}, nil
	}

	offset := pagination.Offset(page, limit)
	result := &SearchResult{}
	hasNext := false

	switch searchType {
	case "users":
		users, err := s.userRepo.SearchByUsername(ctx, query, limit+1, offset)
		if err != nil {
			return nil, pagination.Meta{}, fmt.Errorf("search users: %w", err)
		}
		if len(users) > limit {
			hasNext = true
			users = users[:limit]
		}
		result.Users = toUserShortList(users)

	case "posts":
		posts, err := s.postRepo.SearchByCaption(ctx, query, limit+1, offset)
		if err != nil {
			return nil, pagination.Meta{}, fmt.Errorf("search posts: %w", err)
		}
		if len(posts) > limit {
			hasNext = true
			posts = posts[:limit]
		}
		result.Posts = posts

	default: // "all"
		users, err := s.userRepo.SearchByUsername(ctx, query, limit+1, offset)
		if err != nil {
			s.logger.Warn("search users in all", zap.Error(err))
		} else {
			if len(users) > limit {
				hasNext = true
				users = users[:limit]
			}
			result.Users = toUserShortList(users)
		}

		posts, err := s.postRepo.SearchByCaption(ctx, query, limit+1, offset)
		if err != nil {
			s.logger.Warn("search posts in all", zap.Error(err))
		} else {
			if len(posts) > limit {
				hasNext = true
				posts = posts[:limit]
			}
			result.Posts = posts
		}
	}

	meta := pagination.Meta{
		Page:        page,
		Limit:       limit,
		HasNextPage: hasNext,
	}

	return result, meta, nil
}
