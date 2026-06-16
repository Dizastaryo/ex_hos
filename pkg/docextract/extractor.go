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

// cp1251Rune converts a Windows-1251 byte (0x80–0xFF) to its Unicode rune.
// The 0xC0–0xFF block maps directly to Cyrillic А–я (U+0410–U+044F).
func cp1251Rune(b byte) rune {
	if b >= 0xC0 {
		return rune(0x0410 + int(b-0xC0))
	}
	// 0x80–0xBF: mixed punctuation, Cyrillic extras (ё Ё, №, etc.)
	table := [64]rune{
		0x0402, 0x0403, 0x201A, 0x0453, 0x201E, 0x2026, 0x2020, 0x2021, // 80-87
		0x20AC, 0x2030, 0x0409, 0x2039, 0x040A, 0x040C, 0x040B, 0x040F, // 88-8F
		0x0452, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, // 90-97
		0x0000, 0x2122, 0x0459, 0x203A, 0x045A, 0x045C, 0x045B, 0x045F, // 98-9F
		0x00A0, 0x040E, 0x045E, 0x0408, 0x00A4, 0x0490, 0x00A6, 0x00A7, // A0-A7
		0x0401, 0x00A9, 0x0404, 0x00AB, 0x00AC, 0x00AD, 0x00AE, 0x0407, // A8-AF
		0x00B0, 0x00B1, 0x0406, 0x0456, 0x0491, 0x00B5, 0x00B6, 0x00B7, // B0-B7
		0x0451, 0x2116, 0x0454, 0x00BB, 0x0458, 0x0405, 0x0455, 0x0457, // B8-BF
	}
	return table[b-0x80]
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
// DOCX = ZIP + word/document.xml. Текст в <w:t>; разрывы абзацев после <w:p>.

func extractDocx(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			return extractDocxContent(f)
		}
	}
	return "", fmt.Errorf("word/document.xml not found")
}

func extractDocxContent(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var sb strings.Builder
	dec := xml.NewDecoder(rc)
	dec.Strict = false
	inT := false

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
			if t.Name.Local == "t" {
				inT = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inT = false
			case "p": // paragraph break — newline after each <w:p>
				sb.WriteByte('\n')
			}
		case xml.CharData:
			if inT {
				sb.Write(t)
			}
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

// ─── RTF ──────────────────────────────────────────────────────────────────────
// RTF: state-machine парсер. \'XX декодируется как cp1251 → UTF-8 (критично
// для кириллицы). Контрольные слова пропускаются; \par/\line/\sect/\page → \n.

var wsCollapseRe = regexp.MustCompile(`[ \t]+`)

func extractRTF(data []byte) (string, error) {
	s := string(data)
	n := len(s)
	var sb strings.Builder

	for i := 0; i < n; {
		ch := s[i]
		switch {
		case ch == '{' || ch == '}':
			i++
		case ch == '\\':
			i++
			if i >= n {
				break
			}
			next := s[i]
			switch {
			case next == '\'':
				// \'XX — hex-encoded byte in current code page (cp1251)
				i++
				if i+2 <= n {
					b, err := strconv.ParseUint(s[i:i+2], 16, 8)
					if err == nil {
						bv := byte(b)
						if bv >= 0x80 {
							if r := cp1251Rune(bv); r != 0 {
								sb.WriteRune(r)
							}
						} else if bv >= 0x20 {
							sb.WriteByte(bv)
						}
					}
					i += 2
				}
			case next == '\\' || next == '{' || next == '}':
				sb.WriteByte(next)
				i++
			case next == '\n' || next == '\r':
				// \<newline> — paragraph delimiter in RTF
				sb.WriteByte('\n')
				i++
			case next == '*':
				// \* — ignorable destination; skip
				i++
			case (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z'):
				// Control word: read letters
				start := i
				for i < n && ((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z')) {
					i++
				}
				word := s[start:i]
				// Skip optional numeric parameter (sign + digits)
				if i < n && (s[i] == '-' || (s[i] >= '0' && s[i] <= '9')) {
					if s[i] == '-' {
						i++
					}
					for i < n && s[i] >= '0' && s[i] <= '9' {
						i++
					}
				}
				// Skip optional trailing space (delimiter)
				if i < n && s[i] == ' ' {
					i++
				}
				// Paragraph / line / section / page breaks → newline
				switch word {
				case "par", "pard", "line", "sect", "page":
					sb.WriteByte('\n')
				}
			default:
				i++
			}
		case ch == '\n' || ch == '\r':
			// Literal newlines in RTF source are insignificant
			i++
		default:
			if ch >= 0x20 {
				sb.WriteByte(ch)
			}
			i++
		}
	}

	clean := wsCollapseRe.ReplaceAllString(sb.String(), " ")
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

// slideFileIndex extracts the numeric index from "ppt/slides/slideN.xml".
func slideFileIndex(name string) int {
	base := name[strings.LastIndex(name, "/")+1:] // "slideN.xml"
	base = strings.TrimSuffix(base, ".xml")       // "slideN"
	base = strings.TrimPrefix(base, "slide")      // "N"
	n, err := strconv.Atoi(base)
	if err != nil {
		return -1
	}
	return n
}

func extractPPTX(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	// Собираем файлы слайдов, сортируем по числовому индексу (не лексикографически!)
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
		return slideFileIndex(slideFiles[i].Name) < slideFileIndex(slideFiles[j].Name)
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
// ODP = ZIP + content.xml. Слайды — <draw:page>; текст в <text:p> внутри них.
// Каждый слайд предваряется маркером [Слайд N] для Flutter-карусели.

func extractODP(data []byte) (string, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, f := range r.File {
		if f.Name == "content.xml" {
			return extractODPContent(f)
		}
	}
	return "", fmt.Errorf("content.xml not found in ODP")
}

func extractODPContent(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	var sb strings.Builder
	dec := xml.NewDecoder(rc)
	dec.Strict = false

	depth := 0           // overall element depth
	pageStartDepth := -1 // depth at which current draw:page started (-1 = not in page)
	pDepth := 0          // nesting count of <text:p> elements
	slideNum := 0        // current slide number

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
			depth++
			local := t.Name.Local
			if local == "page" && pageStartDepth < 0 {
				pageStartDepth = depth
							slideNum++
				fmt.Fprintf(&sb, "[Слайд %d]\n", slideNum)
			} else if pageStartDepth >= 0 && local == "p" {
				pDepth++
			}
		case xml.EndElement:
			local := t.Name.Local
			if pageStartDepth >= 0 {
				if local == "page" && depth == pageStartDepth {
					pageStartDepth = -1
					pDepth = 0
					sb.WriteString("\n\n")
				} else if local == "p" && pDepth > 0 {
					pDepth--
					sb.WriteByte('\n')
				}
			}
			depth--
		case xml.CharData:
			if pDepth > 0 {
				sb.Write(t)
			}
		}
	}
	return strings.TrimSpace(sb.String()), nil
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
