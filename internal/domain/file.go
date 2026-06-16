package domain

import (
	"errors"
	"time"
)

var (
	ErrFileNotFound = errors.New("file not found")
)

type FileCategory struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type File struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	Filename       string    `json:"filename"`
	Title          string    `json:"title"`
	AuthorName     string    `json:"author_name"`
	Language       string    `json:"language"`
	FileURL        string    `json:"file_url"`
	MimeType       string    `json:"mime_type"`
	FileSize       int64     `json:"file_size"`
	CategoryID     string    `json:"category_id"`
	DownloadsCount int       `json:"downloads_count"`
	LikesCount     int       `json:"likes_count"`
	ViewsCount     int       `json:"views_count"`
	RatingsCount   int       `json:"ratings_count"`
	RatingsSum     int       `json:"ratings_sum"`
	// UserRating is populated per-viewer by the handler (0 = not rated).
	UserRating     int       `json:"user_rating,omitempty"`
	IsPreviewable  bool      `json:"is_previewable"`
	PreviewURL     string    `json:"preview_url"`
	CoverURL       string    `json:"cover_url"`
	Description    string    `json:"description"`
	PagesCount     int       `json:"pages_count"`
	DocFormat      string    `json:"doc_format"`
	PdfCacheURL         string `json:"pdf_cache_url,omitempty"`
	PdfConversionStatus string `json:"pdf_conversion_status,omitempty"`
	CreatedAt      time.Time `json:"created_at"`

	User     *UserShort    `json:"user,omitempty"`
	Category *FileCategory `json:"category,omitempty"`
	// IsLiked заполняется handler'ом по запросу viewer'а, не сканируется из БД.
	IsLiked bool `json:"is_liked,omitempty"`
	// ReadingStatus заполняется handler'ом: want | reading | done | ""
	ReadingStatus string `json:"reading_status,omitempty"`
	// ExtractedText используется только при Create — не включается в JSON-ответы.
	// Клиент получает текст через GET /files/:id/text.
	ExtractedText string `json:"-"`
}

// AverageRating returns the average star rating, or 0 if no ratings yet.
func (f *File) AverageRating() float64 {
	if f.RatingsCount == 0 {
		return 0
	}
	return float64(f.RatingsSum) / float64(f.RatingsCount)
}

// FileListParams — параметры запроса GET /files
type FileListParams struct {
	CategoryID string
	Q          string // поисковый запрос (tsvector)
	AuthorName string // фильтр/поиск по автору (ILIKE)
	ExcludeID  string // исключить файл (для "похожих")
	DocFormat  string // фильтр по формату: pdf, epub, docx и т.д.
	Language   string // фильтр по языку: ru, en, kk и т.д.
	Sort       string // date | likes | downloads | title
	Cursor     string // UUID последнего файла предыдущей страницы
	Limit      int
}

// UpdateFileMetaRequest — поля доступные для редактирования автором.
type UpdateFileMetaRequest struct {
	Title       string `json:"title"`
	AuthorName  string `json:"author_name"`
	Description string `json:"description"`
	CategoryID  string `json:"category_id"`
	Language    string `json:"language"`
}

type CreateFileRequest struct {
	Filename    string `json:"filename" validate:"required,min=1,max=500"`
	FileURL     string `json:"file_url" validate:"required"`
	MimeType    string `json:"mime_type" validate:"required"`
	FileSize    int64  `json:"file_size" validate:"required,min=1"`
	CategoryID  string `json:"category_id" validate:"omitempty"`
	Description string `json:"description" validate:"omitempty,max=1000"`
	Title       string `json:"title" validate:"omitempty,max=500"`
	AuthorName  string `json:"author_name" validate:"omitempty,max=500"`
	Language    string `json:"language" validate:"omitempty,max=20"`
}
