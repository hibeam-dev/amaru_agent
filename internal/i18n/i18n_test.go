package i18n

import (
	"os"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	origEnv := map[string]string{
		"LANG":        os.Getenv("LANG"),
		"LC_ALL":      os.Getenv("LC_ALL"),
		"LC_MESSAGES": os.Getenv("LC_MESSAGES"),
		"LANGUAGE":    os.Getenv("LANGUAGE"),
	}

	defer func() {
		for key, val := range origEnv {
			_ = os.Setenv(key, val)
		}
	}()

	clearEnvVars := func() {
		for key := range origEnv {
			_ = os.Setenv(key, "")
		}
	}

	tests := []struct {
		name     string
		envSetup map[string]string
		want     string
	}{
		{
			name:     "LANG variable",
			envSetup: map[string]string{"LANG": "es_ES.UTF-8"},
			want:     "es",
		},
		{
			name:     "LC_ALL variable",
			envSetup: map[string]string{"LC_ALL": "fr_FR.UTF-8"},
			want:     "fr",
		},
		{
			name:     "LC_MESSAGES variable",
			envSetup: map[string]string{"LC_MESSAGES": "de_DE.UTF-8"},
			want:     "de",
		},
		{
			name:     "LANGUAGE variable",
			envSetup: map[string]string{"LANGUAGE": "it_IT:en_US:en"},
			want:     "it",
		},
		{
			name:     "LANGUAGE variable with colon syntax",
			envSetup: map[string]string{"LANGUAGE": "pt_BR:pt:en"},
			want:     "pt",
		},
		{
			name:     "Variable precedence (LANG over LC_ALL)",
			envSetup: map[string]string{"LANG": "es_ES.UTF-8", "LC_ALL": "fr_FR.UTF-8"},
			want:     "es",
		},
		{
			name:     "No language set",
			envSetup: map[string]string{},
			want:     DefaultLanguage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnvVars()

			for key, val := range tt.envSetup {
				_ = os.Setenv(key, val)
			}

			got := DetectLanguage()
			if got != tt.want {
				t.Errorf("DetectLanguage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTranslation(t *testing.T) {
	if Bundle == nil {
		t.Skip("Bundle not initialized, skipping test")
	}

	SetLanguage("en")
	testError := "test error"

	testCases := []struct {
		name       string
		messageID  string
		data       map[string]any
		expected   string
		shouldFail bool
	}{
		{
			name:       "Basic translation",
			messageID:  "connection_error",
			data:       map[string]any{"Error": testError},
			expected:   "SSH connection error: test error",
			shouldFail: false,
		},
		{
			name:       "Unknown message ID",
			messageID:  "unknown_message_id",
			data:       nil,
			expected:   "unknown_message_id", // Should return messageID as fallback
			shouldFail: false,
		},
		{
			name:       "Translation with nil data",
			messageID:  "ssh_connection_established",
			data:       nil,
			expected:   "SSH connection established",
			shouldFail: false,
		},
	}

	for _, tc := range testCases {
		t.Run("T/"+tc.name, func(t *testing.T) {
			result := T(tc.messageID, tc.data)

			if tc.shouldFail {
				if result != tc.messageID {
					t.Errorf("Expected message ID as fallback for failing test, got '%s'", result)
				}
			} else {
				if tc.expected != "" && result != tc.expected {
					t.Errorf("Expected '%s', got '%s'", tc.expected, result)
				}
				if result == tc.messageID && tc.messageID != tc.expected {
					t.Errorf("Translation failed, got message ID '%s'", result)
				}
			}
		})
	}

}
