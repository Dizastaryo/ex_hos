package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// dailyPrompts is a fixed deterministic rotation. Same day-of-year ⇒ same
// prompt across the whole user base ⇒ shared social moment ("today everyone
// answers the same question").
//
// To extend: just append. Order isn't sensitive (rotation is stable per-day
// anyway), but try to keep tone consistent with existing entries (curious,
// open-ended, single line in Russian).
var dailyPrompts = []string{
	"что вас удивило\nсегодня?",
	"какой звук\nделает ваш день?",
	"кто заставил вас\nулыбнуться?",
	"что сейчас\nна столе у вас?",
	"какая мысль\nне отпускает?",
	"если бы час\nоткладывался — куда?",
	"что вы\nсейчас слышите?",
	"что хотите\nпопробовать впервые?",
	"какое место\nкажется вашим?",
	"что в этом\nдне нового?",
	"какую песню\nставите на повтор?",
	"кому позвонили бы\nпрямо сейчас?",
	"какой кадр\nостался в голове?",
	"что вас\nвдохновило?",
	"какой запах\nнапоминает что-то?",
	"что сегодня\nбыло вкусным?",
	"какая книга\nрядом с вами?",
	"что заставило\nвас подумать?",
	"куда хотели бы\nпойти прямо сейчас?",
	"что было\nкрасивым сегодня?",
	"какая фраза\nзацепила?",
	"что вы\nсегодня узнали?",
	"какое утро\nу вас было?",
	"что важно\nпрямо сейчас?",
	"кто рядом\nс вами сейчас?",
	"какая мелочь\nпорадовала?",
	"чего ждёте\nна этой неделе?",
	"что попробовать\nна этих выходных?",
	"какой совет\nдали бы себе?",
	"что повторите\nзавтра?",
	"какое чувство\nсейчас?",
}

type DailyPromptHandler struct{}

func NewDailyPromptHandler() *DailyPromptHandler {
	return &DailyPromptHandler{}
}

// GetDailyPrompt godoc
// GET /api/v1/daily-prompt
//
// Returns the prompt for *today* (UTC day-of-year). Deterministic, idempotent,
// no DB. Frontend should cache by the returned `date` and refetch on day flip.
func (h *DailyPromptHandler) GetDailyPrompt(c *fiber.Ctx) error {
	now := time.Now().UTC()
	idx := now.YearDay() % len(dailyPrompts)
	return respondSuccess(c, fiber.StatusOK, fiber.Map{
		"prompt": dailyPrompts[idx],
		"date":   now.Format("2006-01-02"),
	}, nil)
}
