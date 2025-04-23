package i18n

import "embed"

//go:embed locales/*.toml
var LocaleFS embed.FS

func InitDefaultFS() error {
	lang := DetectLanguage()
	return InitWithFS(LocaleFS, lang)
}
