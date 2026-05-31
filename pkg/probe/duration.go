// Package probe wraps ffprobe to extract media duration. Used by:
//   - MediaService.Upload — populate DurationSeconds for audio/video right
//     after the blob lands on disk.
//   - AudioHandler.CreateTrack — fallback when the frontend sends 0 because
//     it couldn't await AudioPlayer.duration on submit (lossy UX).
//
// Returns 0 on any failure (binary missing, bad container, parse error) —
// caller treats 0 as "unknown" and may keep the user-provided value.
package probe

import (
	"os/exec"
	"strconv"
	"strings"
)

// DurationSeconds runs `ffprobe` on the given local path and returns rounded
// integer seconds. Path is whatever the OS resolves (relative or absolute).
//
// ffprobe must be on PATH. If not, returns 0 silently.
func DurationSeconds(path string) int {
	out, err := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=nokey=1:noprint_wrappers=1",
		path,
	).Output()
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0
	}
	return int(f + 0.5) // round
}
