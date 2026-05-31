package handler

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUploadPath_AcceptsValidPaths(t *testing.T) {
	cases := []string{
		"/uploads/2026/05/07/abc.mp4",
		"/uploads/thumbs/x.jpg",
		"/uploads/a.jpg",
	}
	root, err := filepath.Abs(filepath.Join(".", "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range cases {
		got, err := resolveUploadPath(in)
		if err != nil {
			t.Errorf("%q: unexpected error: %v", in, err)
			continue
		}
		if !strings.HasPrefix(got, root+string(filepath.Separator)) {
			t.Errorf("%q: result %q not under uploads root %q", in, got, root)
		}
	}
}

func TestResolveUploadPath_RejectsTraversal(t *testing.T) {
	cases := []string{
		"/uploads/../etc/passwd",
		"/uploads/../../secret",
		"/uploads/foo/../../../boot.ini",
		"/uploads/../",
		"/etc/passwd",
		"/passwd",
		"",
		"/uploads/",
		"/UPLOADS/file.jpg",                 // case-sensitive on prefix
		"/uploads/foo\\..\\..\\etc\\passwd", // backslash injection
	}
	for _, in := range cases {
		if _, err := resolveUploadPath(in); err == nil {
			t.Errorf("%q: expected error, got nil", in)
		}
	}
}
