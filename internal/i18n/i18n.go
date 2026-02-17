// Package i18n provides internationalization support for the Trala dashboard.
// It handles loading translation files and providing localized strings.
package i18n

import (
	"log"
	"os"
	"path/filepath"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.yaml.in/yaml/v4"
	"golang.org/x/text/language"

	"server/internal/config"
)

// Translation directory path
const translationDir = "/app/translations"

// Default fallback language
const fallbackLang = "en"

// Global bundle and default localizer
var (
	bundle    *i18n.Bundle
	localizer *i18n.Localizer
)

// Init initializes the i18n bundle and loads the appropriate translation file.
// It falls back to English if the desired language file is missing.
// This must be called after config.Load() has been executed.
func Init() {
	// Get the language from environment configuration
	lang := config.GetLanguage()
	if lang == "" {
		log.Printf("Language not set - using fallback language: %s", fallbackLang)
		lang = fallbackLang
	}

	// Build the path to the translation file for the selected language
	translationFile := filepath.Join(translationDir, lang+".yaml")
	log.Printf("Attempting to load translation file: %s", translationFile)

	// Check if the translation file exists
	if _, err := os.Stat(translationFile); os.IsNotExist(err) {
		log.Printf("Translation file not found for language '%s': %s", lang, translationFile)

		// Fallback to default language if the desired file is missing
		lang = fallbackLang
		translationFile = filepath.Join(translationDir, lang+".yaml")
		log.Printf("Falling back to default translation file: %s", translationFile)

		// If fallback file is also missing, terminate the application
		if _, err := os.Stat(translationFile); os.IsNotExist(err) {
			log.Fatalf("FATAL: Fallback translation file also not found: %s", translationFile)
			return
		}
	}

	log.Printf("Language set to: %s", lang)

	// Create a new i18n bundle with the selected language
	bundle = i18n.NewBundle(language.Make(lang))

	// Register the YAML unmarshal function to read translation files
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	// Load the translation file into the bundle
	if _, err := bundle.LoadMessageFile(translationFile); err != nil {
		log.Fatalf("Failed to load translation file '%s': %v", translationFile, err)

		// Create a localizer for the current language
		localizer = i18n.NewLocalizer(bundle, lang)
	}
}

// T is a helper function for localization. It takes a message ID and returns the localized string.
// If the localization fails, it returns the message ID as a fallback.
func T(id string) string {
	if localizer == nil {
		return id
	}
	msg, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: id})
	if err != nil {
		// If localization fails, return the message ID as a fallback.
		return id
	}
	return msg
}

// GetLocalizer returns a new localizer for the specified language.
// This is useful for per-request localization in HTTP handlers.
func GetLocalizer(lang string) *i18n.Localizer {
	if bundle == nil {
		return nil
	}
	return i18n.NewLocalizer(bundle, lang)
}

// GetBundle returns the global i18n bundle.
// This can be used for advanced localization scenarios.
func GetBundle() *i18n.Bundle {
	return bundle
}

// GetDefaultLocalizer returns the default localizer initialized during Init().
func GetDefaultLocalizer() *i18n.Localizer {
	return localizer
}

// LocalizeFunc is a template function that can be used with html/template.
// It takes a localizer and message ID, returning the localized string.
func LocalizeFunc(loc *i18n.Localizer, id string) string {
	if loc == nil {
		return id
	}
	msg, err := loc.Localize(&i18n.LocalizeConfig{MessageID: id})
	if err != nil {
		return id
	}
	return msg
}
