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
	"erlang-solutions.com/cortex_agent/internal/i18n"
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

func Connect(ctx context.Context, config config.Config) (Connection, error) {
	key, err := os.ReadFile(config.SSH.KeyFile)
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrSSHConnect,
			i18n.T("ssh_key_error", map[string]interface{}{"Error": err}), err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrSSHConnect,
			i18n.T("ssh_key_error", map[string]interface{}{"Error": err}), err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.SSH.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         config.SSH.Timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Use known_hosts file
	}

	addr := net.JoinHostPort(config.SSH.Host, strconv.Itoa(config.SSH.Port))

	msg := i18n.T("ssh_dialing", map[string]interface{}{
		"Host": config.SSH.Host,
		"Port": config.SSH.Port,
	})
	log.Printf("%s", msg)

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrSSHConnect, i18n.T("ssh_connect_failed", map[string]interface{}{"Address": addr}), err)
	}

	log.Println(i18n.T("ssh_connection_established", nil))
	log.Println(i18n.T("ssh_server_auth", nil))

	cleanup := func() {
		if cerr := client.Close(); cerr != nil {
			msg := i18n.T("connection_close_error", map[string]interface{}{"Error": cerr})
			log.Printf("%s", msg)
		}
	}

	session, err := client.NewSession()
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSession, i18n.T("ssh_session_failed", nil), err)
	}

	sessionCleanup := cleanup
	cleanup = func() {
		if cerr := session.Close(); cerr != nil {
			msg := i18n.T("connection_close_error", map[string]interface{}{"Error": cerr})
			log.Printf("%s", msg)
		}
		sessionCleanup()
	}

	err = session.RequestSubsystem(Subsystem)
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSubsystem,
			i18n.T("ssh_subsystem_failed", map[string]interface{}{"Subsystem": Subsystem}), err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSubsystem, i18n.T("ssh_stdin_failed", nil), err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSubsystem, i18n.T("ssh_stdout_failed", nil), err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		cleanup()
		return nil, errors.WrapWithBase(errors.ErrSSHSubsystem, i18n.T("ssh_stderr_failed", nil), err)
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

	if len(errs) > 0 {
		return errors.Join(errs...)
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
	return c.client
}

func (c *Conn) SendPayload(payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin == nil {
		return errors.New(i18n.T("connection_closed_payload", nil))
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	data = append(data, '\n')

	_, err = c.stdin.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write payload: %w", err)
	}

	return nil
}
