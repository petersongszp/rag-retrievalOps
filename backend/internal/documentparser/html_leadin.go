package documentparser

import (
	"bytes"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

const (
	maxHTMLLeadInBlocks = 8
	maxHTMLLeadInBytes  = 4096
)

type htmlLeadInBlock struct {
	Text   string
	Strong bool
}

func RestoreHTMLLeadInBeforeFirstHeading(fileType string, sourceHTML []byte, markdown string) string {
	fileType = NormalizeFileType(fileType)
	if fileType != "html" && fileType != "htm" {
		return markdown
	}
	blocks := extractHTMLLeadInBlocks(sourceHTML)
	if len(blocks) == 0 {
		return markdown
	}

	comparableMarkdown := comparableLeadInText(markdown)
	missing := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Text == "" {
			continue
		}
		if needle := comparableLeadInText(block.Text); needle != "" && strings.Contains(comparableMarkdown, needle) {
			continue
		}
		missing = append(missing, block.Markdown())
	}
	if len(missing) == 0 {
		return markdown
	}

	prefix := strings.Join(missing, "\n\n")
	content := strings.TrimSpace(markdown)
	if content == "" {
		return prefix
	}
	return prefix + "\n\n" + content
}

func (b htmlLeadInBlock) Markdown() string {
	if b.Strong {
		return "**" + b.Text + "**"
	}
	return b.Text
}

func extractHTMLLeadInBlocks(sourceHTML []byte) []htmlLeadInBlock {
	if len(bytes.TrimSpace(sourceHTML)) == 0 {
		return nil
	}
	root, err := html.Parse(bytes.NewReader(sourceHTML))
	if err != nil {
		return nil
	}
	body := findHTMLElement(root, "body")
	if body == nil {
		body = root
	}

	blocks := make([]htmlLeadInBlock, 0, 2)
	stopped := false
	totalBytes := 0
	for child := body.FirstChild; child != nil && !stopped; child = child.NextSibling {
		collectHTMLLeadInBlocks(child, &blocks, &stopped, &totalBytes)
	}
	return blocks
}

func collectHTMLLeadInBlocks(node *html.Node, blocks *[]htmlLeadInBlock, stopped *bool, totalBytes *int) {
	if node == nil || *stopped || len(*blocks) >= maxHTMLLeadInBlocks || *totalBytes >= maxHTMLLeadInBytes {
		return
	}
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if isHTMLHeadingElement(tag) {
			*stopped = true
			return
		}
		if isHTMLIgnoredTextContainer(tag) {
			return
		}
		if tag == "p" {
			text := normalizeHTMLText(htmlNodeText(node))
			if text != "" {
				*blocks = append(*blocks, htmlLeadInBlock{
					Text:   text,
					Strong: htmlNodeHasStrongSignal(node),
				})
				*totalBytes += len(text)
			}
			return
		}
	}

	for child := node.FirstChild; child != nil && !*stopped; child = child.NextSibling {
		collectHTMLLeadInBlocks(child, blocks, stopped, totalBytes)
	}
}

func findHTMLElement(node *html.Node, tag string) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, tag) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findHTMLElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func isHTMLHeadingElement(tag string) bool {
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	default:
		return false
	}
}

func isHTMLIgnoredTextContainer(tag string) bool {
	switch tag {
	case "script", "style", "noscript", "template":
		return true
	default:
		return false
	}
}

func htmlNodeText(node *html.Node) string {
	var builder strings.Builder
	appendHTMLNodeText(&builder, node)
	return builder.String()
}

func appendHTMLNodeText(builder *strings.Builder, node *html.Node) {
	if node == nil {
		return
	}
	if node.Type == html.TextNode {
		builder.WriteString(node.Data)
		builder.WriteByte(' ')
		return
	}
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if isHTMLIgnoredTextContainer(tag) {
			return
		}
		if tag == "br" {
			builder.WriteByte('\n')
			return
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendHTMLNodeText(builder, child)
	}
}

func normalizeHTMLText(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	if value == "\ufeff" {
		return ""
	}
	return value
}

func htmlNodeHasStrongSignal(node *html.Node) bool {
	if node == nil {
		return false
	}
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if tag == "strong" || tag == "b" {
			return true
		}
		for _, attr := range node.Attr {
			if strings.EqualFold(attr.Key, "style") && styleHasBoldFontWeight(attr.Val) {
				return true
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if htmlNodeHasStrongSignal(child) {
			return true
		}
	}
	return false
}

func styleHasBoldFontWeight(style string) bool {
	compact := strings.ToLower(style)
	compact = strings.ReplaceAll(compact, " ", "")
	compact = strings.ReplaceAll(compact, "\t", "")
	return strings.Contains(compact, "font-weight:bold") ||
		strings.Contains(compact, "font-weight:700") ||
		strings.Contains(compact, "font-weight:800") ||
		strings.Contains(compact, "font-weight:900")
}

func comparableLeadInText(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsSpace(r) || r == '\ufeff' {
			continue
		}
		switch r {
		case '*', '_', '`', '#':
			continue
		default:
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
