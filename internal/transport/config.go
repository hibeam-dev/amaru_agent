package transport

type ConnectionConfig struct {
	Protocol string
	Timeout  string
}

type ConfigPayload struct {
	Application ApplicationConfig `json:"application"`
}

type ApplicationConfig struct {
	Hostname string            `json:"hostname"`
	Port     int               `json:"port"`
	IP       string            `json:"ip,omitempty"`
	Tags     map[string]string `json:"tags"`
	Security map[string]bool   `json:"security"`
	Tunnel   bool              `json:"tunnel"`
}
