package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/seeu/backend/internal/middleware"
	"github.com/seeu/backend/internal/repository/postgres"
)

// AIDailyMaskLimit — сколько масок юзер может генерить за сутки.
const AIDailyMaskLimit = 5

// AIDailyStylizeLimit — стилизаций готовых фото за сутки.
const AIDailyStylizeLimit = 3

// AIDailyCaptionLimit — caption-generations за сутки.
const AIDailyCaptionLimit = 20

type AIHandler struct {
	aiMasksRepo *postgres.AIMasksRepository
	aiStylRepo  *postgres.AIStylizationsRepository
	logger      *zap.Logger
}

// NewAIHandler — старая сигнатура для backward-совместимости с уже
// зарегистрированным /ai/generate-filter (он не требует БД).
func NewAIHandler() *AIHandler {
	return &AIHandler{}
}

// NewAIHandlerWithDeps — расширенный конструктор для endpoint'ов которым
// нужен доступ к БД (Mask/Stylize endpoints персистят историю + rate-limit).
func NewAIHandlerWithDeps(
	masksRepo *postgres.AIMasksRepository,
	stylRepo *postgres.AIStylizationsRepository,
	logger *zap.Logger,
) *AIHandler {
	return &AIHandler{
		aiMasksRepo: masksRepo,
		aiStylRepo:  stylRepo,
		logger:      logger,
	}
}

type GenerateFilterRequest struct {
	Prompt       string `json:"prompt"`
	Style        string `json:"style"`
	BaseImageURL string `json:"base_image_url,omitempty"`
}

type GenerateFilterResponse struct {
	ID          string    `json:"id"`
	Prompt      string    `json:"prompt"`
	Description string    `json:"description,omitempty"`
	ResultURL   string    `json:"result_url"`
	Style       string    `json:"style"`
	CreatedAt   time.Time `json:"created_at"`
}

// GenerateFilter godoc
// POST /api/v1/ai/generate-filter
func (h *AIHandler) GenerateFilter(c *fiber.Ctx) error {
	var req GenerateFilterRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if req.Prompt == "" {
		return respondError(c, fiber.StatusBadRequest, "prompt is required")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return respondError(c, fiber.StatusInternalServerError, "AI service not configured")
	}

	// Build system prompt based on style
	systemPrompt := "You are a creative AI that generates visual effects for a social media app called SeeU. "
	switch req.Style {
	case "mask":
		systemPrompt += "Generate a face mask/filter description. Describe the visual effect in detail for AR overlay."
	case "filter":
		systemPrompt += "Generate a photo filter description. Describe color grading, contrast, and mood adjustments."
	case "sticker":
		systemPrompt += "Generate a sticker/overlay description. Describe the decorative element."
	case "background":
		systemPrompt += "Generate a virtual background description."
	default:
		systemPrompt += "Generate a creative visual effect description."
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}

	// Step 1: Use GPT-4o to refine the prompt into a detailed description
	description, err := h.refinePrompt(httpClient, apiKey, systemPrompt, req.Prompt)
	if err != nil {
		return respondError(c, fiber.StatusInternalServerError, "AI service error")
	}

	// Step 2: Generate image with DALL-E 3
	style := req.Style
	if style == "" {
		style = "effect"
	}

	dallePrompt := fmt.Sprintf(
		"Create a %s effect for a social media camera app: %s. "+
			"Style: modern, clean, warm tones matching SeeU brand (coral #FF5A3C, amber #FFB547). "+
			"PNG with transparency where appropriate.",
		style, description,
	)

	imageURL, err := h.generateImage(httpClient, apiKey, dallePrompt)
	if err != nil {
		// Graceful fallback: return the description without an image URL
		return respondSuccess(c, fiber.StatusOK, GenerateFilterResponse{
			ID:          uuid.New().String(),
			Prompt:      req.Prompt,
			Description: description,
			ResultURL:   "",
			Style:       style,
			CreatedAt:   time.Now().UTC(),
		}, nil)
	}

	return respondSuccess(c, fiber.StatusOK, GenerateFilterResponse{
		ID:          uuid.New().String(),
		Prompt:      req.Prompt,
		Description: description,
		ResultURL:   imageURL,
		Style:       style,
		CreatedAt:   time.Now().UTC(),
	}, nil)
}

// refinePrompt sends the user prompt to GPT-4o and returns a detailed effect description.
func (h *AIHandler) refinePrompt(client *http.Client, apiKey, systemPrompt, userPrompt string) (string, error) {
	body := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens": 200,
	}

	respData, err := h.doOpenAIRequest(client, apiKey, "https://api.openai.com/v1/chat/completions", body)
	if err != nil {
		return "", err
	}

	choices, ok := respData["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return userPrompt, nil // fall back to raw prompt
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return userPrompt, nil
	}
	msg, ok := choice["message"].(map[string]interface{})
	if !ok {
		return userPrompt, nil
	}
	content, _ := msg["content"].(string)
	if content == "" {
		return userPrompt, nil
	}
	return content, nil
}

// generateImage calls the DALL-E 3 images/generations endpoint and returns the image URL.
func (h *AIHandler) generateImage(client *http.Client, apiKey, prompt string) (string, error) {
	body := map[string]interface{}{
		"model":   "dall-e-3",
		"prompt":  prompt,
		"n":       1,
		"size":    "1024x1024",
		"quality": "standard",
	}

	respData, err := h.doOpenAIRequest(client, apiKey, "https://api.openai.com/v1/images/generations", body)
	if err != nil {
		return "", err
	}

	data, ok := respData["data"].([]interface{})
	if !ok || len(data) == 0 {
		return "", fmt.Errorf("no image data in response")
	}
	item, ok := data[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected image data format")
	}
	url, _ := item["url"].(string)
	if url == "" {
		return "", fmt.Errorf("empty image URL in response")
	}
	return url, nil
}

// doOpenAIRequest marshals body, POSTs to url, and decodes the JSON response.
func (h *AIHandler) doOpenAIRequest(client *http.Client, apiKey, url string, body interface{}) (map[string]interface{}, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Surface API-level errors
	if errObj, hasErr := result["error"]; hasErr {
		return nil, fmt.Errorf("openai api error: %v", errObj)
	}

	return result, nil
}

// ===========================================================================
// AI Mask endpoints
// ===========================================================================

type GenerateMaskRequest struct {
	Prompt string `json:"prompt"`
}

type AIMaskResponse struct {
	ID        string    `json:"id"`
	Prompt    string    `json:"prompt"`
	FileURL   string    `json:"file_url"`
	CreatedAt time.Time `json:"created_at"`
}

// GenerateMask godoc
// POST /api/v1/ai/mask  body: {"prompt": "cat ears with sparkles"}
//
// Поток:
//  1. Проверка rate-limit (≤ AIDailyMaskLimit/24h на юзера)
//  2. DALL-E 3 с маска-специфичным промптом (transparent PNG, face accessory)
//  3. Скачивание result-URL в /uploads/ai/masks/<uuid>.png
//  4. Insert в ai_masks, return {id, file_url, created_at}
func (h *AIHandler) GenerateMask(c *fiber.Ctx) error {
	if h.aiMasksRepo == nil {
		return respondError(c, fiber.StatusInternalServerError, "AI handler not configured")
	}
	userID := middleware.GetUserID(c)

	var req GenerateMaskRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if len(req.Prompt) < 3 || len(req.Prompt) > 300 {
		return respondError(c, fiber.StatusBadRequest,
			"prompt должен быть от 3 до 300 символов")
	}

	// Rate-limit
	count, err := h.aiMasksRepo.CountInLast24h(c.Context(), userID)
	if err != nil {
		h.logger.Error("count ai_masks", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "db error")
	}
	if count >= AIDailyMaskLimit {
		return respondError(c, fiber.StatusTooManyRequests,
			fmt.Sprintf("лимит %d генераций в сутки исчерпан", AIDailyMaskLimit))
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return respondError(c, fiber.StatusServiceUnavailable,
			"AI service not configured (нужен OPENAI_API_KEY на сервере)")
	}

	httpClient := &http.Client{Timeout: 90 * time.Second}

	// Маска-специфичный промпт — заставляет DALL-E генерить PNG-style overlay
	// с transparent-вокруг-объекта, оптимизированный под лицо.
	dallePrompt := fmt.Sprintf(
		"A wearable face accessory or mask featuring: %s. "+
			"Style: clean centered illustration on pure black background, "+
			"front-facing view, isolated subject, no human face visible, "+
			"no text, no watermark. Designed as AR-overlay sticker. "+
			"Vibrant colors matching warm-coral palette.",
		req.Prompt,
	)

	imageURL, err := h.generateImage(httpClient, apiKey, dallePrompt)
	if err != nil {
		h.logger.Error("dalle generate", zap.Error(err))
		return respondError(c, fiber.StatusBadGateway, "не удалось сгенерировать маску")
	}

	// Скачиваем результат локально — DALL-E URL'ы expire через час.
	localPath, err := h.downloadToLocal(imageURL, "uploads/ai/masks")
	if err != nil {
		h.logger.Error("download ai mask", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "не удалось сохранить маску")
	}
	fileURL := "/" + localPath

	mask, err := h.aiMasksRepo.Insert(c.Context(), userID, req.Prompt, fileURL)
	if err != nil {
		h.logger.Error("insert ai_mask", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "db error")
	}

	return respondSuccess(c, fiber.StatusCreated, AIMaskResponse{
		ID:        mask.ID,
		Prompt:    mask.Prompt,
		FileURL:   mask.FileURL,
		CreatedAt: mask.CreatedAt,
	}, nil)
}

// ListMasks godoc
// GET /api/v1/ai/masks?limit=30
func (h *AIHandler) ListMasks(c *fiber.Ctx) error {
	if h.aiMasksRepo == nil {
		return respondError(c, fiber.StatusInternalServerError, "AI handler not configured")
	}
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 30)
	masks, err := h.aiMasksRepo.ListByUser(c.Context(), userID, limit)
	if err != nil {
		h.logger.Error("list ai_masks", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "db error")
	}
	if masks == nil {
		masks = []postgres.AIMask{}
	}
	resp := make([]AIMaskResponse, 0, len(masks))
	for _, m := range masks {
		resp = append(resp, AIMaskResponse{
			ID:        m.ID,
			Prompt:    m.Prompt,
			FileURL:   m.FileURL,
			CreatedAt: m.CreatedAt,
		})
	}
	return respondSuccess(c, fiber.StatusOK, resp, nil)
}

// DeleteMask godoc
// DELETE /api/v1/ai/masks/:id
func (h *AIHandler) DeleteMask(c *fiber.Ctx) error {
	if h.aiMasksRepo == nil {
		return respondError(c, fiber.StatusInternalServerError, "AI handler not configured")
	}
	userID := middleware.GetUserID(c)
	id := c.Params("id")
	fileURL, err := h.aiMasksRepo.Delete(c.Context(), id, userID)
	if err != nil {
		// Может быть not-found (другому юзеру / уже удалено) — отдаём 404.
		return respondError(c, fiber.StatusNotFound, "маска не найдена")
	}
	// Best-effort cleanup blob'а — ошибки игнорируем (как в media-cleanup'е).
	if filepath.IsAbs(fileURL) || len(fileURL) > 1 {
		// Strip leading '/' → relative path относительно cwd.
		path := fileURL
		if path[0] == '/' {
			path = path[1:]
		}
		_ = os.Remove(path)
	}
	return respondSuccess(c, fiber.StatusOK, fiber.Map{"ok": true}, nil)
}

// downloadToLocal качает src URL → uploads/<destDir>/<uuid>.png, возвращает
// relative path. Используется для остаточного хранения DALL-E URL'ов которые
// expir'ятся через час.
func (h *AIHandler) downloadToLocal(srcURL, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	resp, err := http.Get(srcURL)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("source returned non-200")
	}
	name := uuid.New().String() + ".png"
	full := filepath.Join(destDir, name)
	f, err := os.Create(full)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("copy bytes: %w", err)
	}
	// Возвращаем посредник без leading '/' — caller сам добавит.
	return filepath.ToSlash(full), nil
}

// ===========================================================================
// AI Stylize endpoints (применить «киношный» стиль к снятому фото)
// ===========================================================================

// stylePromptTemplates — preset'ы превращающие текстовый стиль в детальный
// промпт для DALL-E с правильным набором визуальных дескрипторов.
var stylePromptTemplates = map[string]string{
	"ghibli":    "Studio Ghibli anime style, soft watercolor textures, painterly details, lush nature, warm pastel palette, hand-drawn aesthetic, Hayao Miyazaki style.",
	"pixar":     "Pixar 3D animation style, smooth subsurface scattering, expressive cartoon proportions, cinematic studio lighting, vibrant colors, family-friendly look.",
	"anime":     "Modern anime illustration, sharp line art, cel-shaded coloring, large expressive eyes, dynamic background, Japanese animation style.",
	"watercolor": "Watercolor painting, soft brush strokes, paper texture visible, gentle color bleeds, artistic composition, delicate translucency.",
	"cyberpunk": "Cyberpunk aesthetic, neon magenta and electric blue lighting, rainy night street vibe, holographic details, gritty futuristic atmosphere.",
	"oilpainting": "Oil painting style, visible brush strokes, rich colors, classical portrait composition, museum-quality finish.",
}

type StylizeRequest struct {
	ImageURL string `json:"image_url"` // server-relative /uploads/... либо absolute http://...
	Style    string `json:"style"`     // preset id из stylePromptTemplates, либо 'custom'
	Prompt   string `json:"prompt"`    // обязателен если style == 'custom'
}

type StylizeResponse struct {
	ID        string    `json:"id"`
	SourceURL string    `json:"source_url"`
	ResultURL string    `json:"result_url"`
	Style     string    `json:"style"`
	Prompt    string    `json:"prompt"`
	CreatedAt time.Time `json:"created_at"`
}

// Stylize godoc
// POST /api/v1/ai/stylize  body: {"image_url":"/uploads/...","style":"ghibli"}
//                     или: {"image_url":"...","style":"custom","prompt":"...как Ван Гог"}
//
// Sync-flow (DALL-E отвечает за 10-25s): handler ждёт ответ openai и
// возвращает уже скачанный result-url. Шаги:
//  1. Rate-limit ≤ AIDailyStylizeLimit/24h
//  2. GPT-4o-vision описывает source-картинку (≈ "молодая женщина, рыжие волосы, портрет")
//  3. Composed prompt = "<description>. <style template>" → DALL-E 3
//  4. Download result → /uploads/ai/stylized/<uuid>.png
//  5. Insert ai_stylizations, return response
func (h *AIHandler) Stylize(c *fiber.Ctx) error {
	if h.aiStylRepo == nil {
		return respondError(c, fiber.StatusInternalServerError, "AI handler not configured")
	}
	userID := middleware.GetUserID(c)

	var req StylizeRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.ImageURL == "" {
		return respondError(c, fiber.StatusBadRequest, "image_url is required")
	}
	style := req.Style
	if style == "" {
		style = "custom"
	}

	var styleText string
	if style == "custom" {
		if len(req.Prompt) < 3 || len(req.Prompt) > 300 {
			return respondError(c, fiber.StatusBadRequest,
				"prompt должен быть от 3 до 300 символов")
		}
		styleText = req.Prompt
	} else {
		tmpl, ok := stylePromptTemplates[style]
		if !ok {
			return respondError(c, fiber.StatusBadRequest,
				"неизвестный style; используйте custom + prompt")
		}
		styleText = tmpl
	}

	count, err := h.aiStylRepo.CountInLast24h(c.Context(), userID)
	if err != nil {
		h.logger.Error("count ai_stylizations", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "db error")
	}
	if count >= AIDailyStylizeLimit {
		return respondError(c, fiber.StatusTooManyRequests,
			fmt.Sprintf("лимит %d стилизаций в сутки исчерпан", AIDailyStylizeLimit))
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return respondError(c, fiber.StatusServiceUnavailable,
			"AI service not configured (нужен OPENAI_API_KEY)")
	}

	httpClient := &http.Client{Timeout: 90 * time.Second}

	// Source URL → absolute. Если пришёл server-relative, добавим host.
	sourceAbs := req.ImageURL
	if len(sourceAbs) > 0 && sourceAbs[0] == '/' {
		// Используем заголовок Host если есть, иначе fallback.
		host := c.Hostname()
		if host == "" {
			host = "localhost:8001"
		}
		scheme := "http"
		if c.Protocol() == "https" {
			scheme = "https"
		}
		sourceAbs = fmt.Sprintf("%s://%s%s", scheme, host, sourceAbs)
	}

	// Step 1: GPT-4o-vision — описание source-картинки.
	description, err := h.describeImage(httpClient, apiKey, sourceAbs)
	if err != nil {
		h.logger.Warn("describe failed, using generic prompt", zap.Error(err))
		description = "subject in original photo"
	}

	// Step 2: Compose prompt + DALL-E.
	dallePrompt := fmt.Sprintf(
		"Reimagine the subject of a photo (%s) in this style: %s "+
			"Keep the subject's overall identity recognizable. "+
			"Full-frame composition, no text, no watermark.",
		description, styleText,
	)
	imageURL, err := h.generateImage(httpClient, apiKey, dallePrompt)
	if err != nil {
		h.logger.Error("dalle generate", zap.Error(err))
		return respondError(c, fiber.StatusBadGateway, "не удалось сгенерировать стилизацию")
	}

	// Step 3: Download → local.
	localPath, err := h.downloadToLocal(imageURL, "uploads/ai/stylized")
	if err != nil {
		h.logger.Error("download stylized", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "не удалось сохранить результат")
	}
	resultURL := "/" + localPath

	rec, err := h.aiStylRepo.Insert(c.Context(), userID, req.ImageURL, resultURL, style, styleText)
	if err != nil {
		h.logger.Error("insert ai_stylization", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "db error")
	}

	return respondSuccess(c, fiber.StatusCreated, StylizeResponse{
		ID:        rec.ID,
		SourceURL: rec.SourceURL,
		ResultURL: rec.ResultURL,
		Style:     rec.Style,
		Prompt:    rec.Prompt,
		CreatedAt: rec.CreatedAt,
	}, nil)
}

// describeImage — GPT-4o-vision коротко описывает что на картинке. Нужно
// чтобы DALL-E генерировал результат «по мотивам» source'а (DALL-E
// не принимает картинки как input для генерации, только text).
func (h *AIHandler) describeImage(client *http.Client, apiKey, imageURL string) (string, error) {
	body := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]interface{}{
			{
				"role": "system",
				"content": "Describe the main subject of the image in one short sentence " +
					"(person/object, basic appearance). No interpretation, no judgement. " +
					"English, max 25 words.",
			},
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "image_url", "image_url": map[string]string{"url": imageURL}},
				},
			},
		},
		"max_tokens": 80,
	}
	respData, err := h.doOpenAIRequest(client, apiKey, "https://api.openai.com/v1/chat/completions", body)
	if err != nil {
		return "", err
	}
	choices, ok := respData["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "", errors.New("no choices")
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return "", errors.New("bad choice format")
	}
	msg, ok := choice["message"].(map[string]interface{})
	if !ok {
		return "", errors.New("no message")
	}
	content, _ := msg["content"].(string)
	if content == "" {
		return "", errors.New("empty content")
	}
	return content, nil
}

// ===========================================================================
// AI Caption (vision-LLM описывает фото → 3 caption'а + 5 хэштегов)
// ===========================================================================

type CaptionRequest struct {
	ImageURL string `json:"image_url"` // server-relative или absolute
	Vibe     string `json:"vibe"`      // optional: "casual", "poetic", "funny" — meta-tone
}

type CaptionResponse struct {
	Captions []string `json:"captions"`
	Hashtags []string `json:"hashtags"`
}

// GenerateCaption godoc
// POST /api/v1/ai/caption  body: {"image_url":"/uploads/...","vibe":"casual"}
//
// GPT-4o-vision получает картинку + system-prompt, возвращает JSON с
// caption[3] и hashtag[5] для русскоязычной аудитории. Один запрос на
// OpenAI, ~3-5s.
func (h *AIHandler) GenerateCaption(c *fiber.Ctx) error {
	var req CaptionRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.ImageURL == "" {
		return respondError(c, fiber.StatusBadRequest, "image_url is required")
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return respondError(c, fiber.StatusServiceUnavailable,
			"AI service not configured (нужен OPENAI_API_KEY)")
	}

	// Absolute URL (DALL-E/vision требует доступной картинки).
	sourceAbs := req.ImageURL
	if len(sourceAbs) > 0 && sourceAbs[0] == '/' {
		host := c.Hostname()
		if host == "" {
			host = "localhost:8001"
		}
		scheme := "http"
		if c.Protocol() == "https" {
			scheme = "https"
		}
		sourceAbs = fmt.Sprintf("%s://%s%s", scheme, host, sourceAbs)
	}

	vibe := req.Vibe
	if vibe == "" {
		vibe = "casual"
	}

	systemPrompt := `Ты копирайтер русскоязычного Instagram-приложения. Получив картинку:
1. Сгенерируй 3 разных caption'а к посту: короткие (до 80 символов), на русском, ` + vibe + ` тон, без банальностей.
2. Сгенерируй 5 релевантных хэштегов на русском или английском (одним словом, без #).

Верни СТРОГО валидный JSON: {"captions":[...3 строки...],"hashtags":[...5 строк...]}. Никакого markdown, никаких комментариев.`

	httpClient := &http.Client{Timeout: 30 * time.Second}
	body := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "image_url", "image_url": map[string]string{"url": sourceAbs}},
				},
			},
		},
		"max_tokens":      400,
		"response_format": map[string]string{"type": "json_object"},
	}
	respData, err := h.doOpenAIRequest(httpClient, apiKey, "https://api.openai.com/v1/chat/completions", body)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("caption openai", zap.Error(err))
		}
		return respondError(c, fiber.StatusBadGateway, "AI service error")
	}

	choices, ok := respData["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return respondError(c, fiber.StatusBadGateway, "empty response")
	}
	choice, _ := choices[0].(map[string]interface{})
	msg, _ := choice["message"].(map[string]interface{})
	content, _ := msg["content"].(string)
	if content == "" {
		return respondError(c, fiber.StatusBadGateway, "empty content")
	}

	// Parse inner JSON
	var parsed CaptionResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// Fallback: попробуем выдрать вручную если модель что-то лишнее обернула
		return respondError(c, fiber.StatusBadGateway,
			"AI вернул некорректный JSON")
	}
	if len(parsed.Captions) == 0 {
		return respondError(c, fiber.StatusBadGateway, "no captions generated")
	}
	// Sanitize hashtags — убираем leading '#' если модель всё-таки решила добавить.
	for i, h := range parsed.Hashtags {
		if len(h) > 0 && h[0] == '#' {
			parsed.Hashtags[i] = h[1:]
		}
	}
	return respondSuccess(c, fiber.StatusOK, parsed, nil)
}

// ListStylizations godoc
// GET /api/v1/ai/stylizations?limit=30
func (h *AIHandler) ListStylizations(c *fiber.Ctx) error {
	if h.aiStylRepo == nil {
		return respondError(c, fiber.StatusInternalServerError, "AI handler not configured")
	}
	userID := middleware.GetUserID(c)
	limit := c.QueryInt("limit", 30)
	list, err := h.aiStylRepo.ListByUser(c.Context(), userID, limit)
	if err != nil {
		h.logger.Error("list ai_stylizations", zap.Error(err))
		return respondError(c, fiber.StatusInternalServerError, "db error")
	}
	if list == nil {
		list = []postgres.AIStylization{}
	}
	resp := make([]StylizeResponse, 0, len(list))
	for _, s := range list {
		resp = append(resp, StylizeResponse{
			ID:        s.ID,
			SourceURL: s.SourceURL,
			ResultURL: s.ResultURL,
			Style:     s.Style,
			Prompt:    s.Prompt,
			CreatedAt: s.CreatedAt,
		})
	}
	return respondSuccess(c, fiber.StatusOK, resp, nil)
}
