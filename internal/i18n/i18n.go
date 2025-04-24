package i18n

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
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
		return fmt.Errorf("failed to read locales directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join("locales", entry.Name())

			_, err := Bundle.LoadMessageFileFS(fs, filePath)
			if err != nil {
				return fmt.Errorf("failed to load message file %s: %w", filePath, err)
			}
		}
	}

	SetLanguage(defaultLang)
	return nil
}

// Initializes the i18n system using files from a directory on disk
func InitWithDirectory(dirPath string, defaultLang string) error {
	Bundle = initBundle()

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read locales directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(dirPath, entry.Name())

			_, err := Bundle.LoadMessageFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to load message file %s: %w", filePath, err)
			}
		}
	}

	SetLanguage(defaultLang)
	return nil
}

func SetLanguage(lang string) {
	ActiveLocalizer = i18n.NewLocalizer(Bundle, lang, DefaultLanguage)
}

func T(messageID string, templateData map[string]interface{}) string {
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

func Tf(messageID string, templateData map[string]interface{}) string {
	return T(messageID, templateData)
}

func Tp(messageID string, pluralCount interface{}, templateData map[string]interface{}) string {
	if ActiveLocalizer == nil {
		return messageID
	}

	if templateData == nil {
		templateData = make(map[string]interface{})
	}

	templateData["Count"] = pluralCount

	lc := &i18n.LocalizeConfig{
		MessageID:    messageID,
		PluralCount:  pluralCount,
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
