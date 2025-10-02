package i18n

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/hibeam-dev/amaru_agent/internal/util"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

const DefaultLanguage = "en"

var Bundle *i18n.Bundle
var ActiveLocalizer *i18n.Localizer

func initBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	return bundle
}

// Initializes the i18n system using an embedded filesystem
func InitWithFS(fs embed.FS, defaultLang string) error {
	Bundle = initBundle()

	entries, err := fs.ReadDir("locales")
	if err != nil {
		return util.NewError(util.ErrTypeConfig, "failed to read locales directory", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join("locales", entry.Name())

			_, err := Bundle.LoadMessageFileFS(fs, filePath)
			if err != nil {
				return util.NewError(util.ErrTypeConfig, fmt.Sprintf("failed to load message file %s", filePath), err)
			}
		}
	}

	SetLanguage(defaultLang)
	return nil
}

func SetLanguage(lang string) {
	ActiveLocalizer = i18n.NewLocalizer(Bundle, lang, DefaultLanguage)
}

func T(messageID string, templateData map[string]any) string {
	if ActiveLocalizer == nil {
		return messageID
	}

	lc := &i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	}

	msg, err := ActiveLocalizer.Localize(lc)
	if err != nil {
		return messageID
	}

	return msg
}

func DetectLanguage() string {
	envVars := []string{"LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE"}

	for _, env := range envVars {
		lang := os.Getenv(env)
		if lang == "" {
			continue
		}

		parts := strings.Split(lang, ":")
		if len(parts) == 0 || parts[0] == "" {
			continue
		}

		subParts := strings.Split(parts[0], "_")
		if len(subParts) > 0 && subParts[0] != "" {
			return subParts[0]
		}
	}

	// No language was detected
	return DefaultLanguage
}
