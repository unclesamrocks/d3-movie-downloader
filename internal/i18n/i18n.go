package i18n

import (
	"embed"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localeFS embed.FS

// fieldPattern matches {{.FieldName}} in templates
var fieldPattern = regexp.MustCompile(`\{\{\.\s*(\w+)\s*\}\}`)

// messageFields stores expected template fields per message ID
type messageFields map[string][]string

// Localizer provides message translation for different languages
type Localizer struct {
	bundle  *i18n.Bundle
	fields  messageFields
	english *i18n.Localizer
}

// NewLocalizer creates a new Localizer and loads all locale files
func NewLocalizer() (*Localizer, error) {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	fields := make(messageFields)

	// Load all embedded locale files
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return nil, fmt.Errorf("failed to read locales directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := localeFS.ReadFile("locales/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("failed to read locale file %s: %w", entry.Name(), err)
		}
		if _, err := bundle.ParseMessageFileBytes(data, entry.Name()); err != nil {
			return nil, fmt.Errorf("failed to parse locale file %s: %w", entry.Name(), err)
		}

		// Extract template fields from English locale (source of truth)
		if strings.Contains(entry.Name(), "en") {
			extractFields(data, fields)
		}
	}

	return &Localizer{
		bundle:  bundle,
		fields:  fields,
		english: i18n.NewLocalizer(bundle, "en"),
	}, nil
}

// extractFields parses TOML and collects {{.Field}} names per message ID
func extractFields(data []byte, fields messageFields) {
	var parsed map[string]map[string]any
	if err := toml.Unmarshal(data, &parsed); err != nil {
		log.Printf("i18n: failed to extract fields: %v", err)
		return
	}

	for msgID, section := range parsed {
		other, ok := section["other"]
		if !ok {
			continue
		}
		text, ok := other.(string)
		if !ok {
			continue
		}
		matches := fieldPattern.FindAllStringSubmatch(text, -1)
		if len(matches) == 0 {
			continue
		}
		var fieldNames []string
		for _, m := range matches {
			fieldNames = append(fieldNames, m[1])
		}
		sort.Strings(fieldNames)
		fields[msgID] = fieldNames
	}
}

// Localize returns the translated message for the given language and message ID.
// templateData is optional; pass nil for messages without variables.
func (l *Localizer) Localize(lang string, messageID string, templateData map[string]any) string {
	l.validateTemplateData(messageID, templateData)

	loc := i18n.NewLocalizer(l.bundle, lang)

	msgConfig := &i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	}

	msg, err := loc.Localize(msgConfig)
	if err != nil {
		// Fallback: try English
		if lang != "en" {
			msg, err = l.english.Localize(msgConfig)
			if err != nil {
				return messageID // Last resort: return key itself
			}
		} else {
			return messageID
		}
	}

	return msg
}

// validateTemplateData checks that provided keys match expected template fields
func (l *Localizer) validateTemplateData(messageID string, data map[string]any) {
	expected, ok := l.fields[messageID]
	if !ok {
		// Message has no template fields
		if len(data) > 0 {
			log.Printf("i18n: [%s] message has no template fields, but got %d extra keys: %v",
				messageID, len(data), mapKeys(data))
		}
		return
	}

	if len(data) == 0 && len(expected) > 0 {
		log.Printf("i18n: [%s] missing required fields: %v", messageID, expected)
		return
	}

	// Check for missing fields
	dataKeys := mapKeys(data)
	for _, field := range expected {
		if !contains(dataKeys, field) {
			log.Printf("i18n: [%s] missing field: %s", messageID, field)
		}
	}

	// Check for extra/misspelled fields
	for key := range data {
		if !contains(expected, key) {
			log.Printf("i18n: [%s] unexpected field: %s (possible typo?)", messageID, key)
		}
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
