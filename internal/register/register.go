package register

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"erlang-solutions.com/amaru_agent/internal/i18n"
	"erlang-solutions.com/amaru_agent/internal/util"
)

type Client interface {
	RegisterWithBackend(token string, publicKeyPath string, backendHost string) error
}

type client struct{}

func NewClient() Client {
	return &client{}
}

func (c *client) RegisterWithBackend(token string, publicKeyPath string, backendHost string) error {
	publicKey, err := readPublicKey(publicKeyPath)
	if err != nil {
		return fmt.Errorf("%s", i18n.T("register_key_read_error", map[string]any{"Error": err}))
	}

	registrationData := map[string]string{
		"token":  token,
		"pubkey": string(publicKey),
	}

	jsonData, err := json.Marshal(registrationData)
	if err != nil {
		return fmt.Errorf("%s", i18n.T("register_json_error", map[string]any{"Error": err}))
	}

	registrationURL := backendHost + "/api/v1/agent/register"
	resp, err := http.Post(registrationURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("%s", i18n.T("register_request_error", map[string]any{"Error": err}))
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			util.Warn("Failed to close response body", map[string]any{"component": "api", "error": closeErr})
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s", i18n.T("register_response_read_error", map[string]any{"Error": err}))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("%s", i18n.T("register_response_error", map[string]any{"StatusCode": resp.StatusCode, "Body": string(body)}))
	}

	return nil
}

func readPublicKey(keyPath string) ([]byte, error) {
	return os.ReadFile(keyPath)
}
