package i18n

import (
	"os"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	origLang := os.Getenv("LANG")
	origLCAll := os.Getenv("LC_ALL")

	defer func() {
		_ = os.Setenv("LANG", origLang)
		_ = os.Setenv("LC_ALL", origLCAll)
	}()

	_ = os.Setenv("LANG", "es_ES.UTF-8")
	_ = os.Setenv("LC_ALL", "")
	lang := DetectLanguage()
	if lang != "es" {
		t.Errorf("Expected 'es' from LANG, got '%s'", lang)
	}

	_ = os.Setenv("LANG", "")
	_ = os.Setenv("LC_ALL", "fr_FR.UTF-8")
	lang = DetectLanguage()
	if lang != "fr" {
		t.Errorf("Expected 'fr' from LC_ALL, got '%s'", lang)
	}

	_ = os.Setenv("LANG", "")
	_ = os.Setenv("LC_ALL", "")
	lang = DetectLanguage()
	if lang != DefaultLanguage {
		t.Errorf("Expected default language '%s', got '%s'", DefaultLanguage, lang)
	}
}

func TestTranslation(t *testing.T) {
	if Bundle == nil {
		t.Skip("Bundle not initialized, skipping test")
	}

	SetLanguage("en")
	msg := T("connection_error", map[string]interface{}{"Error": "test error"})
	if msg == "connection_error" {
		t.Errorf("Translation failed, got message ID")
	}

	msg = T("unknown_message_id", nil)
	if msg != "unknown_message_id" {
		t.Errorf("Expected message ID as fallback, got '%s'", msg)
	}
}
