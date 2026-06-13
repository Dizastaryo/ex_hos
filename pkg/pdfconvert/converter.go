// Package pdfconvert converts office documents to PDF using LibreOffice headless.
// Supported: docx, rtf, odt, fb2, pptx, odp (any format LibreOffice can open).
//
// Requires libreoffice or soffice to be available in PATH.
// Install on Debian/Ubuntu: apt-get install -y libreoffice --no-install-recommends
package pdfconvert

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// defaultTimeout — максимальное время на конвертацию одного файла.
// Большинство документов конвертируются за 2-5 сек; сложные PPTX — до 30 сек.
const defaultTimeout = 60 * time.Second

// IsAvailable возвращает true если LibreOffice установлен и доступен в PATH.
func IsAvailable() bool {
	_, err := findBinary()
	return err == nil
}

// ConvertToPDF конвертирует файл по пути srcPath в PDF через LibreOffice headless.
// Возвращает байты готового PDF. Временные файлы очищаются внутри функции.
func ConvertToPDF(ctx context.Context, srcPath string) ([]byte, error) {
	binary, err := findBinary()
	if err != nil {
		return nil, err
	}

	// Отдельная временная директория — LibreOffice пишет вывод сюда
	outDir, err := os.MkdirTemp("", "pdfconv_out_*")
	if err != nil {
		return nil, fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(outDir)

	// Таймаут конвертации
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary,
		"--headless",
		"--norestore",
		"--convert-to", "pdf",
		"--outdir", outDir,
		srcPath,
	)
	// LibreOffice иногда пишет прогресс в stderr — собираем для диагностики
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("libreoffice convert failed: %w\noutput: %s", err, out)
	}

	// LibreOffice именует вывод: <basename_без_расширения>.pdf
	base := filepath.Base(srcPath)
	ext := filepath.Ext(base)
	pdfName := base[:len(base)-len(ext)] + ".pdf"
	pdfPath := filepath.Join(outDir, pdfName)

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("output pdf not found after conversion: %w", err)
	}
	return data, nil
}

// findBinary ищет libreoffice или soffice в PATH.
func findBinary() (string, error) {
	for _, name := range []string{"libreoffice", "soffice"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("LibreOffice not found in PATH: install with 'apt-get install libreoffice'")
}
