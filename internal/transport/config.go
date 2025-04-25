package transport

type ConnectionConfig struct {
	Protocol string
	Timeout  string
}

type ConfigPayload struct {
	Application ApplicationConfig `json:"application"`
	Agent       AgentConfig       `json:"agent"`
}

type ApplicationConfig struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

type AgentConfig struct {
	Tags map[string]string `json:"tags"`
}
