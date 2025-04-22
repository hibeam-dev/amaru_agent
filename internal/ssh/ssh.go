package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/pkg/errors"
	"golang.org/x/crypto/ssh"
)

const Subsystem = "cortex"

type Connection interface {
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Stderr() io.Reader
	Client() interface{}
	SendPayload(payload interface{}) error
	Close() error
}

type Conn struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader
	mu      sync.Mutex
}

func Connect(ctx context.Context, config config.Config) (Connection, error) {
	key, err := os.ReadFile(config.SSH.KeyFile)
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrSSHConnect, "failed to read SSH key file", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrSSHConnect, "failed to parse SSH key", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.SSH.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         config.SSH.Timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key verification
	}

	addr := net.JoinHostPort(config.SSH.Host, strconv.Itoa(config.SSH.Port))

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrSSHConnect, fmt.Sprintf("failed to connect to %s", addr), err)
	}

	cleanup := func() {
		if cerr := client.Close(); cerr != nil {
			log.Printf("Error closing SSH client: %v", cerr)
		}
	}

	session, err := client.NewSession()
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSession, "failed to create session", err)
	}

	sessionCleanup := cleanup
	cleanup = func() {
		if cerr := session.Close(); cerr != nil {
			log.Printf("Error closing SSH session: %v", cerr)
		}
		sessionCleanup()
	}

	err = session.RequestSubsystem(Subsystem)
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSubsystem,
			fmt.Sprintf("failed to request subsystem '%s'", Subsystem), err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSubsystem, "failed to get stdin pipe", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSubsystem, "failed to get stdout pipe", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSubsystem, "failed to get stderr pipe", err)
	}

	return &Conn{
		client:  client,
		session: session,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
	}, nil
}

func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error

	// Close stdin first to signal EOF to the remote end
	if c.stdin != nil {
		if err := c.stdin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing stdin: %w", err))
		}
	}

	if c.session != nil {
		if err := c.session.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing session: %w", err))
		}
		c.session = nil
	}

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("error closing client: %w", err))
		}
		c.client = nil
	}

	// Return the first error if any
	if len(errs) > 0 {
		for _, err := range errs[1:] {
			log.Printf("Additional close error: %v", err)
		}
		return errs[0]
	}
	return nil
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

func (c *Conn) SendPayload(payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize payload to JSON: %w", err)
	}

	_, err = c.stdin.Write(jsonData)
	if err != nil {
		return fmt.Errorf("failed to send payload to subsystem: %w", err)
	}

	buffer := make([]byte, 1024)
	n, err := c.stdout.Read(buffer)
	if err != nil {
		return fmt.Errorf("failed to read acknowledgment: %w", err)
	}

	response := string(buffer[:n])
	log.Printf("Received response: %s", response)
	if response != "CONFIG_ACK\n" {
		log.Printf("Unexpected response from subsystem: %q", response)
	}

	return nil
}

func (c *Conn) Stdin() io.WriteCloser {
	return c.stdin
}

func (c *Conn) Stdout() io.Reader {
	return c.stdout
}

func (c *Conn) Stderr() io.Reader {
	return c.stderr
}

func (c *Conn) Client() interface{} {
	if c.client == nil {
		return nil
	}
	return c.client
}
