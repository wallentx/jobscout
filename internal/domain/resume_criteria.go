package domain

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxResumeTextBytes = 120_000

func extractResumeText(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("resume path cannot be empty")
	}
	expanded, err := expandUserPath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(expanded)
	if err != nil {
		return "", fmt.Errorf("read resume: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("resume path is a directory")
	}

	switch strings.ToLower(filepath.Ext(expanded)) {
	case ".doc":
		return textFromLegacyDoc(expanded)
	case ".docx":
		return textFromZippedXML(expanded, "word/document.xml")
	case ".odt":
		return textFromZippedXML(expanded, "content.xml")
	case ".rtf":
		data, err := os.ReadFile(expanded)
		if err != nil {
			return "", fmt.Errorf("read resume: %w", err)
		}
		return stripRTF(string(data)), nil
	case ".pdf":
		return textFromPDF(expanded)
	default:
		data, err := os.ReadFile(expanded)
		if err != nil {
			return "", fmt.Errorf("read resume: %w", err)
		}
		if !utf8.Valid(data) {
			return "", fmt.Errorf("unsupported binary resume format %q", filepath.Ext(expanded))
		}
		return normalizeExtractedText(string(data)), nil
	}
}

func ExtractResumeText(path string) (string, error) {
	return extractResumeText(path)
}

func expandUserPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func textFromZippedXML(path string, innerPath string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open document: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	for _, file := range reader.File {
		if file.Name != innerPath {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("open document text: %w", err)
		}
		defer func() {
			_ = rc.Close()
		}()
		data, err := io.ReadAll(io.LimitReader(rc, maxResumeTextBytes*4))
		if err != nil {
			return "", fmt.Errorf("read document text: %w", err)
		}
		if len(data) == maxResumeTextBytes*4 {
			fmt.Fprintf(os.Stderr, "Warning: Resume is very large. Only the first %d bytes of XML will be processed.\n", maxResumeTextBytes*4)
		}
		return textFromXML(data), nil
	}

	return "", fmt.Errorf("document text part not found")
}

func textFromXML(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var b strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch token := token.(type) {
		case xml.CharData:
			text := strings.TrimSpace(string(token))
			if text != "" {
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(text)
			}
		case xml.StartElement:
			name := strings.ToLower(token.Name.Local)
			if name == "p" || name == "br" || name == "tab" {
				b.WriteByte('\n')
			}
		}
	}
	return normalizeExtractedText(b.String())
}

func textFromPDF(path string) (string, error) {
	if binary, err := exec.LookPath("pdftotext"); err == nil {
		out, err := exec.Command(binary, path, "-").Output()
		if err != nil {
			return "", fmt.Errorf("pdftotext failed: %w", err)
		}
		return normalizeExtractedText(string(out)), nil
	}
	if binary, err := exec.LookPath("mutool"); err == nil {
		out, err := exec.Command(binary, "draw", "-F", "txt", path).Output()
		if err != nil {
			return "", fmt.Errorf("mutool failed: %w", err)
		}
		return normalizeExtractedText(string(out)), nil
	}
	return "", fmt.Errorf("PDF resume support requires pdftotext or mutool on PATH")
}

func textFromLegacyDoc(path string) (string, error) {
	if binary, err := exec.LookPath("antiword"); err == nil {
		out, err := exec.Command(binary, path).Output()
		if err != nil {
			return "", fmt.Errorf("antiword failed: %w", err)
		}
		return normalizeExtractedText(string(out)), nil
	}
	if binary, err := exec.LookPath("catdoc"); err == nil {
		out, err := exec.Command(binary, path).Output()
		if err != nil {
			return "", fmt.Errorf("catdoc failed: %w", err)
		}
		return normalizeExtractedText(string(out)), nil
	}
	return "", fmt.Errorf("DOC resume support requires antiword or catdoc on PATH")
}

func stripRTF(value string) string {
	replacements := map[string]string{
		`\par`: "\n",
		`\tab`: " ",
	}
	for old, replacement := range replacements {
		value = strings.ReplaceAll(value, old, replacement)
	}
	controlWord := regexp.MustCompile(`\\[a-zA-Z]+-?\d* ?`)
	value = controlWord.ReplaceAllString(value, "")
	escaped := regexp.MustCompile(`\\['{}\\]`)
	value = escaped.ReplaceAllStringFunc(value, func(match string) string {
		return strings.TrimPrefix(match, `\`)
	})
	value = strings.NewReplacer("{", " ", "}", " ").Replace(value)
	return normalizeExtractedText(value)
}

func normalizeExtractedText(value string) string {
	value = html.UnescapeString(value)
	var b strings.Builder
	lastSpace := false
	lastNewline := false
	for _, r := range value {
		switch {
		case r == '\n' || r == '\r':
			if !lastNewline {
				b.WriteByte('\n')
			}
			lastSpace = false
			lastNewline = true
		case unicode.IsSpace(r):
			if !lastSpace && !lastNewline {
				b.WriteByte(' ')
			}
			lastSpace = true
		case unicode.IsPrint(r):
			b.WriteRune(r)
			lastSpace = false
			lastNewline = false
		}
	}
	return strings.TrimSpace(b.String())
}
