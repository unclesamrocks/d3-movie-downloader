package i18n

import (
	"embed"
	"fmt"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localeFS embed.FS

// Localizer provides message translation for different languages
type Localizer struct {
	bundle *i18n.Bundle
}

// NewLocalizer creates a new Localizer and loads all locale files
func NewLocalizer() (*Localizer, error) {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

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
	}

	return &Localizer{bundle: bundle}, nil
}

// Localize returns the translated message for the given language and message ID.
// templateData is optional; pass nil for messages without variables.
func (l *Localizer) Localize(lang string, messageID string, templateData map[string]interface{}) string {
	loc := i18n.NewLocalizer(l.bundle, lang)

	msgConfig := &i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	}

	msg, err := loc.Localize(msgConfig)
	if err != nil {
		// Fallback: try English
		if lang != "en" {
			locEn := i18n.NewLocalizer(l.bundle, "en")
			msg, err = locEn.Localize(msgConfig)
			if err != nil {
				return messageID // Last resort: return key itself
			}
		} else {
			return messageID
		}
	}

	return msg
}
