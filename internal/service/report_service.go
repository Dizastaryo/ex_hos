package service

import (
	"context"
	"fmt"

	"github.com/seeu/backend/internal/domain"
	"github.com/seeu/backend/internal/repository/postgres"
	"go.uber.org/zap"
)

type ReportService struct {
	repo   *postgres.ReportRepository
	logger *zap.Logger
}

func NewReportService(repo *postgres.ReportRepository, logger *zap.Logger) *ReportService {
	return &ReportService{repo: repo, logger: logger}
}

// reportRateLimit caps how many reports a single user can file per hour.
// Anything above this is most likely abuse.
const reportRateLimit = 30

func (s *ReportService) Submit(ctx context.Context, reporterID string, req *domain.CreateReportRequest) (*domain.Report, error) {
	count, err := s.repo.CountRecentByReporter(ctx, reporterID)
	if err != nil {
		return nil, err
	}
	if count >= reportRateLimit {
		return nil, fmt.Errorf("report rate limit exceeded")
	}

	report := &domain.Report{
		ReporterID: reporterID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Reason:     req.Reason,
		Details:    req.Details,
	}
	if err := s.repo.Create(ctx, report); err != nil {
		return nil, err
	}
	s.logger.Info("report filed",
		zap.String("reporter_id", reporterID),
		zap.String("target_type", string(req.TargetType)),
		zap.String("target_id", req.TargetID),
		zap.String("reason", string(req.Reason)),
	)
	return report, nil
}
