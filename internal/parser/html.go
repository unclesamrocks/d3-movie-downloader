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

	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonString), &jsonData); err != nil {
		return nil, nil
	}

	// Navigate to post.data.media.url
	result := extractMediaUrl(jsonData)
	if result != nil {
		return result, nil
	}

	// Also try post.data.link.url
	result = extractLinkUrl(jsonData)
	if result != nil {
		return result, nil
	}

	return nil, nil
}

func extractMediaUrl(data map[string]interface{}) *VideoExtractionResult {
	post, ok := data["post"].(map[string]interface{})
	if !ok {
		return nil
	}

	postData, ok := post["data"].(map[string]interface{})
	if !ok {
		return nil
	}

	media, ok := postData["media"].(map[string]interface{})
	if !ok {
		return nil
	}

	mediaURL, ok := media["url"].(string)
	if !ok || mediaURL == "" {
		return nil
	}

	if !IsValidVideoUrl(mediaURL) {
		return nil
	}

	return &VideoExtractionResult{
		VideoURL: mediaURL,
		Width:    getIntFromInterface(media["width"]),
		Height:   getIntFromInterface(media["height"]),
	}
}

func extractLinkUrl(data map[string]interface{}) *VideoExtractionResult {
	post, ok := data["post"].(map[string]interface{})
	if !ok {
		return nil
	}

	postData, ok := post["data"].(map[string]interface{})
	if !ok {
		return nil
	}

	link, ok := postData["link"].(map[string]interface{})
	if !ok {
		return nil
	}

	linkURL, ok := link["url"].(string)
	if !ok || linkURL == "" {
		return nil
	}

	if !IsValidVideoUrl(linkURL) {
		return nil
	}

	return &VideoExtractionResult{
		VideoURL: linkURL,
		Width:    getIntFromInterface(link["width"]),
		Height:   getIntFromInterface(link["height"]),
	}
}

func getIntFromInterface(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case int64:
		return int(val)
	default:
		return 0
	}
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
