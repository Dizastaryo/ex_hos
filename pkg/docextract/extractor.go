// Package docextract извлекает plain-text из читательских форматов документов.
// Используется при загрузке файла в библиотеку — результат сохраняется в
// files.extracted_text и отдаётся Flutter-ридерам для Tier-2 форматов
// (FB2, DOCX, RTF, ODT) и slide-preview для Tier-3 (PPTX, ODP).
//
// Все функции graceful: возвращают "" при ошибке, не ломают upload.
package docextract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const maxExtractBytes = 500 * 1024 // 500 KB — потолок для extracted_text

// ExtractText извлекает plain-text из байт файла по его расширению.
// Поддерживаемые форматы: fb2, docx, rtf, odt, pptx, odp, txt, md.
// Для PDF использовать CountPDFPages; текст PDF не извлекается.
func ExtractText(data []byte, ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	var text string
	var err error
	switch ext {
	case "fb2":
		text, err = extractFB2(data)
	case "docx":
		text, err = extractDocx(data)
	case "rtf":
		text, err = extractRTF(data)
	case "odt":
		text, err = extractODT(data)
	case "pptx":
		text, err = extractPPTX(data)
	case "odp":
		text, err = extractODP(data)
	case "txt", "md":
		// Чистый текст — возвращаем как есть
		return truncate(string(data))
	default:
		return ""
	}
	if err != nil {
		return ""
	}
	return truncate(text)
}

// CountPDFPages подсчитывает страницы PDF сканированием байт.
// Ищет /Count N в page tree (работает для большинства PDF).
func CountPDFPages(data []byte) int {
	re := regexp.MustCompile(`/Count\s+(\d+)`)
	matches := re.FindAllSubmatch(data, -1)
	maxCount := 0
	for _, m := range matches {
		n, _ := strconv.Atoi(string(m[1]))
		if n > maxCount {
			maxCount = n
		}
	}
	return maxCount
}

// DocFormat возвращает нижний регистр расширения без точки.
func DocFormat(filename string) string {
	i := strings.LastIndex(filename, ".")
	if i < 0 || i == len(filename)-1 {
		return ""
	}
	return strings.ToLower(filename[i+1:])
}

// ─── FB2 ──────────────────────────────────────────────────────────────────────
// FictionBook2 — XML. Текст лежит в <p>, <v>, <subtitle>, <epigraph> внутри <body>.

func extractFB2(data []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose

	var sb strings.Builder
	inBody := false
	inBinary := false
	depth := 0

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Пропускаем ошибки — FB2 часто бывает с кривым XML
			if err := dec.Skip(); err != nil {
				break
			}
			continue
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := t.Name.Local
			switch local {
			case "body":
				inBody = true
				depth = 0
			case "binary":
				inBinary = true
			}
			if inBody {
				depth++
			}
		case xml.EndElement:
			local := t.Name.Local
			if local == "binary" {
				inBinary = false
			}
			if inBody {
				depth--
				if local == "p" || local == "v" || local == "subtitle" || local == "title" {
					sb.WriteByte('\n')
				}
				if local == "body" {
					inBody = false
				}
			}
		case xml.CharData:
			if inBody && !inBinary {
				sb.Write(t)
			}
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

// ─── DOCX ─────────────────────────────────────────────────────────────────────
// DOCX = ZIP + word/document.xml. Текст в элементах <w:t>.

func extractDocx(data []byte) (string, error) {
	return extractZipXML(data, "word/document.xml", func(local string) bool {
		return local == "t"
	})
}

// ─── RTF ──────────────────────────────────────────────────────────────────────
// RTF: стрипаем управляющие последовательности регулярками.

var rtfStripRe = regexp.MustCompile(`\\[a-z\*]+-?\d* ?|\\'[0-9a-fA-F]{2}|[{}\\]`)
var wsCollapseRe = regexp.MustCompile(`[ \t]+`)

func extractRTF(data []byte) (string, error) {
	s := string(data)
	// Убираем control words, hex chars, braces, backslashes
	clean := rtfStripRe.ReplaceAllString(s, " ")
	// Схлопываем пробелы
	clean = wsCollapseRe.ReplaceAllString(clean, " ")
	return strings.TrimSpace(clean), nil
}

// ─── ODT ──────────────────────────────────────────────────────────────────────
// ODT = ZIP + content.xml. Текст в <text:p> и <text:h> (заголовки).

func extractODT(data []byte) (string, error) {
	return extractZipXML(data, "content.xml", func(local string) bool {
		return local == "p" || local == "h"
	})
}

// ─── PPTX ─────────────────────────────────────────────────────────────────────
// PPTX = ZIP. Слайды: ppt/slides/slide*.xml. Текст в <a:t>.

func extractPPTX(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	// Собираем файлы слайдов, сортируем по имени
	var slideFiles []*zip.File
	for _, f := range r.File {
		name := f.Name
		if strings.HasPrefix(name, "ppt/slides/slide") &&
			strings.HasSuffix(name, ".xml") &&
			!strings.Contains(name, "_rels") {
			slideFiles = append(slideFiles, f)
		}
	}
	sort.Slice(slideFiles, func(i, j int) bool {
		return slideFiles[i].Name < slideFiles[j].Name
	})

	var sb strings.Builder
	for i, sf := range slideFiles {
		text, err := extractZipFileXML(sf, func(local string) bool { return local == "t" })
		if err != nil || text == "" {
			continue
		}
		fmt.Fprintf(&sb, "[Слайд %d]\n%s\n\n", i+1, text)
	}
	return sb.String(), nil
}

// ─── ODP ──────────────────────────────────────────────────────────────────────
// ODP = ZIP + content.xml. Текст в <text:p>.

func extractODP(data []byte) (string, error) {
	return extractZipXML(data, "content.xml", func(local string) bool {
		return local == "p"
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// extractZipXML открывает zip-архив, находит файл по имени и извлекает текст.
func extractZipXML(data []byte, filename string, isText func(string) bool) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, f := range r.File {
		if f.Name == filename {
			return extractZipFileXML(f, isText)
		}
	}
	return "", fmt.Errorf("file %s not found in zip", filename)
}

// extractZipFileXML читает один файл из zip-архива и возвращает текст
// из XML-элементов, local name которых совпадает с предикатом isText.
// Используется depth-counter: CharData захватывается пока depth > 0.
func extractZipFileXML(f *zip.File, isText func(string) bool) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var sb strings.Builder
	dec := xml.NewDecoder(rc)
	dec.Strict = false
	depth := 0

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if isText(t.Name.Local) {
				depth++
			}
		case xml.EndElement:
			if isText(t.Name.Local) && depth > 0 {
				depth--
				sb.WriteByte('\n')
			}
		case xml.CharData:
			if depth > 0 {
				sb.Write(t)
			}
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

func truncate(s string) string {
	if len(s) > maxExtractBytes {
		return s[:maxExtractBytes]
	}
	return s
}
