package canonical

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"

	"interview-agents/internal/documentparser"
)

const Version = "canonical-normalizer-v1"

var (
	headingLinePattern         = regexp.MustCompile(`^(#{1,6})\s*(.+?)\s*$`)
	numericSectionHeadingStart = regexp.MustCompile(`^\s*\d+(?:\.\d+)*\.?(?:\s+|$)`)
	fragmentHeading            = regexp.MustCompile(`^\s*\d+(?:\.\d+)*\s+[A-Z][A-Z0-9]{1,9}\s*$`)
	numberedTermHeading        = regexp.MustCompile(`^\s*(\d+(?:\.\d+)*)\s+([A-Z][A-Z0-9]{1,9})\s*$`)
	cjkHeadingSpace            = regexp.MustCompile(`([\p{Han}])\s+([\p{Han}])`)
	blankLines                 = regexp.MustCompile(`\n{3,}`)
)

// Normalize converts parser output into deterministic canonical Markdown used
// by chunking and retrieval. It preserves the parser output in
// ContentMarkdownRaw so sidecars remain auditable.
func Normalize(doc *documentparser.NormalizedDocument) (*documentparser.NormalizedDocument, error) {
	if doc == nil {
		return nil, fmt.Errorf("normalized document is nil")
	}
	raw := doc.ContentMarkdownRaw
	if raw == "" {
		raw = doc.ContentMarkdown
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("normalized markdown content is empty")
	}

	applied := make([]string, 0)
	warnings := make([]string, 0)
	content := raw

	content = applyRule(content, normalizeUnicodeAndLineEndings, "unicode-whitespace", &applied)
	content = applyRule(content, normalizeLineNoise, "parser-noise", &applied)
	content = applyRule(content, normalizeHeadings, "heading-cleanup", &applied)
	content = applyRule(content, normalizeHeadingContinuations, "heading-continuation-merge", &applied)
	content = strings.TrimSpace(blankLines.ReplaceAllString(content, "\n\n"))
	content, tables := normalizeTablesAndSpans(content, doc.Tables, &applied)

	out := *doc
	out.ContentMarkdownRaw = raw
	out.ContentMarkdown = content
	out.Tables = tables
	out.Canonicalization = documentparser.CanonicalizationInfo{
		Version:       Version,
		AppliedRules:  uniqueStrings(applied),
		Warnings:      warnings,
		RawSHA1:       sha1Hex(raw),
		CanonicalSHA1: sha1Hex(content),
	}
	return &out, nil
}

func normalizeTablesAndSpans(content string, existing []documentparser.NormalizedTable, applied *[]string) (string, []documentparser.NormalizedTable) {
	normalized, parsed := documentparser.NormalizeMarkdownPipeTables(content)
	if len(parsed) == 0 {
		return content, existing
	}
	if normalized != content || tableSpansChanged(existing, parsed) {
		*applied = append(*applied, "table-span-rebase")
	}
	return normalized, mergeTableMetadata(existing, parsed)
}

func tableSpansChanged(existing, parsed []documentparser.NormalizedTable) bool {
	if len(existing) != len(parsed) {
		return true
	}
	for i := range parsed {
		if existing[i].MarkdownStart != parsed[i].MarkdownStart || existing[i].MarkdownEnd != parsed[i].MarkdownEnd {
			return true
		}
	}
	return false
}

func mergeTableMetadata(existing, parsed []documentparser.NormalizedTable) []documentparser.NormalizedTable {
	merged := make([]documentparser.NormalizedTable, len(parsed))
	for i, table := range parsed {
		merged[i] = table
		if i >= len(existing) {
			continue
		}
		previous := existing[i]
		if previous.ID != "" {
			merged[i].ID = previous.ID
		}
		if previous.Page > 0 {
			merged[i].Page = previous.Page
		}
		merged[i].MergedCells = previous.MergedCells
		merged[i].Nested = previous.Nested
		merged[i].ParentTableID = previous.ParentTableID
		if previous.Quality.Status != "" || len(previous.Quality.Warnings) > 0 {
			merged[i].Quality = previous.Quality
		}
	}
	return merged
}

func applyRule(content string, fn func(string) string, rule string, applied *[]string) string {
	next := fn(content)
	if next != content {
		*applied = append(*applied, rule)
	}
	return next
}

func normalizeUnicodeAndLineEndings(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return norm.NFKC.String(content)
}

func normalizeLineNoise(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ·") {
			trimmed = "## " + strings.TrimSpace(strings.TrimPrefix(trimmed, "## ·"))
		}
		if strings.HasPrefix(trimmed, "- ·") {
			trimmed = "- " + strings.TrimSpace(strings.TrimPrefix(trimmed, "- ·"))
		}
		if strings.HasPrefix(trimmed, "- -") {
			trimmed = "- " + strings.TrimSpace(strings.TrimPrefix(trimmed, "- -"))
		}
		lines[i] = trimmed
	}
	return strings.Join(lines, "\n")
}

func normalizeHeadings(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		level, title, ok := parseHeading(line)
		if !ok {
			continue
		}
		title = normalizeHeadingTitle(title)
		lines[i] = level + " " + title
	}
	return strings.Join(lines, "\n")
}

func normalizeHeadingContinuations(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		level, title, ok := parseHeading(lines[i])
		if !ok {
			out = append(out, lines[i])
			continue
		}

		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		if j >= len(lines) {
			out = append(out, lines[i])
			continue
		}
		nextLevel, nextTitle, nextOK := parseHeading(lines[j])
		if !nextOK || nextLevel != level || !shouldMergeHeading(title, nextTitle) {
			out = append(out, lines[i])
			continue
		}

		merged := level + " " + normalizeHeadingTitle(mergeHeadingTitles(title, nextTitle))
		out = append(out, merged)
		i = j
	}
	return strings.Join(out, "\n")
}

func parseHeading(line string) (string, string, bool) {
	match := headingLinePattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(match) != 3 {
		return "", "", false
	}
	return match[1], strings.TrimSpace(match[2]), true
}

func normalizeHeadingTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, "*_`")
	title = strings.TrimSpace(title)
	title = regexp.MustCompile(`^(\d+(?:\.\d+)*)([^\d\s.])`).ReplaceAllString(title, "$1 $2")
	title = cjkHeadingSpace.ReplaceAllString(title, "$1$2")
	return title
}

func shouldMergeHeading(first, second string) bool {
	first = strings.TrimSpace(first)
	second = strings.TrimSpace(second)
	if first == "" || second == "" {
		return false
	}
	if numericSectionHeadingStart.MatchString(second) {
		return false
	}
	if fragmentHeading.MatchString(first) {
		return true
	}
	if strings.HasSuffix(first, "。") || strings.HasSuffix(first, "：") || strings.HasSuffix(first, ":") {
		return false
	}
	firstRunes := []rune(first)
	secondRunes := []rune(second)
	return len(firstRunes) >= 14 && len(secondRunes) <= 18
}

func mergeHeadingTitles(first, second string) string {
	if match := numberedTermHeading.FindStringSubmatch(first); len(match) == 3 {
		section := match[1]
		term := match[2]
		return section + " " + term + second
	}
	return first + second
}

func sha1Hex(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
