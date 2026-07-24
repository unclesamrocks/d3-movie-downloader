package parser

import (
	"encoding/json"
	"net/url"
	"strings"
)

// VideoExtractionResult holds the extracted video URL and metadata
type VideoExtractionResult struct {
	VideoURL string
	Width    int
	Height   int
}

// d3Media represents the media object in d3.ru JSON
type d3Media struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// d3Link represents the link object in d3.ru JSON
type d3Link struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// d3PostData represents the data object inside a post
type d3PostData struct {
	Media *d3Media `json:"media"`
	Link  *d3Link  `json:"link"`
}

// d3Post represents the post object in d3.ru JSON
type d3Post struct {
	Data d3PostData `json:"data"`
}

// d3PageData represents the top-level JSON structure from d3.ru
type d3PageData struct {
	Post d3Post `json:"post"`
}

// ExtractVideoUrl extracts video URL from d3.ru HTML content
// d3.ru stores page data in: window.entryStorages[window.pageName] = { ...json... }
// Video URL is located at: post.data.media.url
func ExtractVideoUrl(html string) (*VideoExtractionResult, error) {
	// Find the start of JSON data after window.entryStorages[window.pageName] =
	marker := "window.entryStorages[window.pageName] = "
	startIndex := strings.Index(html, marker)
	if startIndex == -1 {
		return nil, nil
	}

	// Find the JSON object starting from the marker
	jsonStart := startIndex + len(marker)

	// Find the end of JSON by counting braces
	braceCount := 0
	jsonEnd := jsonStart
	foundStart := false

	for i := jsonStart; i < len(html); i++ {
		char := html[i]

		if char == '{' {
			braceCount++
			foundStart = true
		} else if char == '}' {
			braceCount--
		}

		if foundStart && braceCount == 0 {
			jsonEnd = i + 1
			break
		}
	}

	if !foundStart || braceCount != 0 {
		return nil, nil
	}

	jsonString := html[jsonStart:jsonEnd]

	var pageData d3PageData
	if err := json.Unmarshal([]byte(jsonString), &pageData); err != nil {
		return nil, nil
	}

	// Try post.data.media.url first
	if media := pageData.Post.Data.Media; media != nil && IsValidVideoUrl(media.URL) {
		return &VideoExtractionResult{
			VideoURL: media.URL,
			Width:    media.Width,
			Height:   media.Height,
		}, nil
	}

	// Also try post.data.link.url
	if link := pageData.Post.Data.Link; link != nil && IsValidVideoUrl(link.URL) {
		return &VideoExtractionResult{
			VideoURL: link.URL,
			Width:    link.Width,
			Height:   link.Height,
		}, nil
	}

	return nil, nil
}

// IsValidVideoUrl checks if a URL is a valid video URL (must end with .mp4)
func IsValidVideoUrl(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return strings.HasSuffix(strings.ToLower(u.Path), ".mp4")
	}
	// Fallback: just check the string ends with .mp4
	return strings.HasSuffix(strings.ToLower(rawURL), ".mp4")
}

// ResolveUrl resolves a potentially relative URL against a base URL
func ResolveUrl(baseUrl, videoUrl string) string {
	u, err := url.Parse(videoUrl)
	if err != nil {
		return videoUrl
	}

	base, err := url.Parse(baseUrl)
	if err != nil {
		return videoUrl
	}

	return base.ResolveReference(u).String()
}

// FindVideoUrl extracts the first video URL from HTML, resolving relative URLs
func FindVideoUrl(html string, baseUrl string) (*VideoExtractionResult, error) {
	result, err := ExtractVideoUrl(html)
	if err != nil || result == nil {
		return nil, err
	}

	// Resolve relative URLs
	resolvedUrl := ResolveUrl(baseUrl, result.VideoURL)

	// Verify it's still an mp4
	if !IsValidVideoUrl(resolvedUrl) {
		return nil, nil
	}

	result.VideoURL = resolvedUrl
	return result, nil
}
