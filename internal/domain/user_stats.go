package domain

import "time"

// UserStats — агрегированные лайки пользователя из всех источников.
// total_likes = сумма всех полей ниже (кроме book_likes считается отдельно от audio).
type UserStats struct {
	UserID       string    `json:"user_id"       db:"user_id"`
	TotalLikes   int       `json:"total_likes"   db:"total_likes"`
	ScannerLikes int       `json:"scanner_likes" db:"scanner_likes"`
	PostLikes    int       `json:"post_likes"    db:"post_likes"`
	StoryLikes   int       `json:"story_likes"   db:"story_likes"`
	ReelLikes    int       `json:"reel_likes"    db:"reel_likes"`
	AudioLikes   int       `json:"audio_likes"   db:"audio_likes"`
	VideoLikes   int       `json:"video_likes"   db:"video_likes"`
	BookLikes    int       `json:"book_likes"    db:"book_likes"`
	UpdatedAt    time.Time `json:"updated_at"    db:"updated_at"`
}

// SocialLevel возвращает уровень пользователя по total_likes (0–5).
func (s *UserStats) SocialLevel() int {
	switch {
	case s.TotalLikes >= 20000:
		return 5
	case s.TotalLikes >= 5000:
		return 4
	case s.TotalLikes >= 1000:
		return 3
	case s.TotalLikes >= 200:
		return 2
	case s.TotalLikes >= 50:
		return 1
	default:
		return 0
	}
}

// SocialLevelName возвращает название уровня.
func (s *UserStats) SocialLevelName() string {
	names := []string{"Новичок", "Известный", "Популярный", "Звезда", "Легенда", "Икона"}
	l := s.SocialLevel()
	if l >= len(names) {
		return names[len(names)-1]
	}
	return names[l]
}

// NextMilestone возвращает следующий порог лайков (-1 если достигнут максимум).
func (s *UserStats) NextMilestone() int {
	milestones := []int{50, 200, 1000, 5000, 20000, -1}
	return milestones[s.SocialLevel()]
}
