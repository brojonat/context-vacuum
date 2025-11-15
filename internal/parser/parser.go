package parser

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Parser handles content extraction from various sources
type Parser struct {
	maxFileSize int64
	httpClient  *http.Client
}

// NewParser creates a new Parser with explicit configuration
func NewParser(maxFileSize int64) *Parser {
	return &Parser{
		maxFileSize: maxFileSize,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ParseFile reads and returns content from a local file
func (p *Parser) ParseFile(path string) (string, error) {
	// Check file size
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Size() > p.maxFileSize {
		return "", fmt.Errorf("file size %d exceeds max size %d", info.Size(), p.maxFileSize)
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(content), nil
}

// ParseURL fetches and extracts text content from a URL
func (p *Parser) ParseURL(url string) (string, error) {
	resp, err := p.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Check content length
	if resp.ContentLength > p.maxFileSize {
		return "", fmt.Errorf("content length %d exceeds max size %d", resp.ContentLength, p.maxFileSize)
	}

	// Read body with size limit
	limitedReader := io.LimitReader(resp.Body, p.maxFileSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if int64(len(body)) > p.maxFileSize {
		return "", fmt.Errorf("response body exceeds max size %d", p.maxFileSize)
	}

	// If it's HTML, extract text content
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		return p.extractTextFromHTML(string(body))
	}

	return string(body), nil
}

// extractTextFromHTML extracts readable text from HTML content with semantic structure
func (p *Parser) extractTextFromHTML(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var text strings.Builder
	var inPre bool
	var preContent strings.Builder

	// Try to find main content area first
	mainContent := findMainContent(doc)
	if mainContent == nil {
		mainContent = doc
	}

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Skip non-content elements
			switch n.Data {
			case "script", "style", "nav", "header", "footer", "aside", "iframe":
				return
			case "pre", "code":
				// Handle code blocks specially
				if n.Data == "pre" {
					inPre = true
					preContent.Reset()
					preContent.WriteString("\n```\n")
				}
			case "h1", "h2", "h3", "h4", "h5", "h6":
				// Add markdown-style headers
				level := n.Data[1] - '0'
				text.WriteString("\n")
				for i := 0; i < int(level); i++ {
					text.WriteString("#")
				}
				text.WriteString(" ")
			case "p", "div", "article", "section":
				// Add paragraph breaks
				if text.Len() > 0 && !strings.HasSuffix(text.String(), "\n\n") {
					text.WriteString("\n\n")
				}
			case "br":
				text.WriteString("\n")
			case "li":
				text.WriteString("\n- ")
			}
		}

		if n.Type == html.TextNode {
			content := n.Data

			if inPre {
				// Preserve exact formatting in code blocks
				preContent.WriteString(content)
			} else {
				// Clean up whitespace for normal text
				content = strings.TrimSpace(content)
				if content != "" {
					// Check if we need a space before this text
					if text.Len() > 0 && !strings.HasSuffix(text.String(), " ") &&
					   !strings.HasSuffix(text.String(), "\n") {
						text.WriteString(" ")
					}
					text.WriteString(content)
				}
			}
		}

		// Recursively process children
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}

		// Close code blocks
		if n.Type == html.ElementNode && n.Data == "pre" {
			preContent.WriteString("\n```\n")
			text.WriteString(preContent.String())
			inPre = false
		}
	}

	extract(mainContent)

	// Clean up excessive newlines
	result := text.String()
	result = strings.TrimSpace(result)
	// Replace 3+ newlines with just 2
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result, nil
}

// findMainContent attempts to find the main content area of an HTML document
func findMainContent(n *html.Node) *html.Node {
	// Try to find common content containers in order of preference
	contentSelectors := []string{"main", "article", "content", "post", "entry"}

	for _, selector := range contentSelectors {
		if result := findElementByTag(n, selector); result != nil {
			return result
		}
		if result := findElementByClass(n, selector); result != nil {
			return result
		}
		if result := findElementByID(n, selector); result != nil {
			return result
		}
	}

	return nil
}

// findElementByTag finds first element with given tag name
func findElementByTag(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := findElementByTag(c, tag); result != nil {
			return result
		}
	}
	return nil
}

// findElementByClass finds first element with class containing the given string
func findElementByClass(n *html.Node, class string) *html.Node {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "class" && strings.Contains(attr.Val, class) {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := findElementByClass(c, class); result != nil {
			return result
		}
	}
	return nil
}

// findElementByID finds first element with id containing the given string
func findElementByID(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "id" && strings.Contains(attr.Val, id) {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := findElementByID(c, id); result != nil {
			return result
		}
	}
	return nil
}

// ParseBookmarkHTML parses an HTML bookmark file and returns list of bookmarks
func (p *Parser) ParseBookmarkHTML(path string) ([]Bookmark, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read bookmark file: %w", err)
	}

	doc, err := html.Parse(strings.NewReader(string(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var bookmarks []Bookmark
	var extract func(*html.Node)

	extract = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			var href, title string
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href = attr.Val
				}
			}
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				title = n.FirstChild.Data
			}

			if href != "" && title != "" {
				bookmarks = append(bookmarks, Bookmark{
					Title: title,
					URL:   href,
				})
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)
	return bookmarks, nil
}

// Bookmark represents a parsed bookmark entry
type Bookmark struct {
	Title string
	URL   string
}
