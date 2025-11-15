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

// extractTextFromHTML extracts readable text from HTML content
func (p *Parser) extractTextFromHTML(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var text strings.Builder
	var extract func(*html.Node)

	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			// Clean up whitespace
			content := strings.TrimSpace(n.Data)
			if content != "" {
				text.WriteString(content)
				text.WriteString("\n")
			}
		}

		// Skip script and style tags
		if n.Type == html.ElementNode {
			if n.Data == "script" || n.Data == "style" {
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(doc)
	return text.String(), nil
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
