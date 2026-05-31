package domain

import "time"

type ReportTargetType string

const (
	ReportTargetPost    ReportTargetType = "post"
	ReportTargetComment ReportTargetType = "comment"
	ReportTargetStory   ReportTargetType = "story"
	ReportTargetUser    ReportTargetType = "user"
	ReportTargetVideo   ReportTargetType = "video"
)

type ReportReason string

const (
	ReasonSpam       ReportReason = "spam"
	ReasonHarassment ReportReason = "harassment"
	ReasonIllegal    ReportReason = "illegal"
	ReasonNSFW       ReportReason = "nsfw"
	ReasonSelfHarm   ReportReason = "self_harm"
	ReasonOther      ReportReason = "other"
)

type Report struct {
	ID         string           `json:"id"`
	ReporterID string           `json:"reporter_id"`
	TargetType ReportTargetType `json:"target_type"`
	TargetID   string           `json:"target_id"`
	Reason     ReportReason     `json:"reason"`
	Details    string           `json:"details"`
	Status     string           `json:"status"`
	CreatedAt  time.Time        `json:"created_at"`
}

type CreateReportRequest struct {
	TargetType ReportTargetType `json:"target_type" validate:"required,oneof=post comment story user video"`
	TargetID   string           `json:"target_id" validate:"required,uuid"`
	Reason     ReportReason     `json:"reason" validate:"required,oneof=spam harassment illegal nsfw self_harm other"`
	Details    string           `json:"details" validate:"omitempty,max=1000"`
}
