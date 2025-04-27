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
	"time"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/transport"
	"erlang-solutions.com/cortex_agent/internal/util"
	"golang.org/x/crypto/ssh"
)

const Subsystem = "cortex"

var _ transport.Connection = (*Conn)(nil)

type Conn struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader
	mu      sync.Mutex
}

type sshCreator struct{}

func (c *sshCreator) CreateConnection(ctx context.Context, config config.Config, opts map[string]any) (transport.Connection, error) {
	connectionOpts := ConnectOptions{
		User:    config.Connection.User,
		Host:    config.Connection.Host,
		Port:    config.Connection.Port,
		KeyFile: config.Connection.KeyFile,
		Timeout: config.Connection.Timeout,
	}

	if keyFile, ok := opts["key_file"].(string); ok && keyFile != "" {
		connectionOpts.KeyFile = keyFile
	}
	if user, ok := opts["user"].(string); ok && user != "" {
		connectionOpts.User = user
	}
	if host, ok := opts["host"].(string); ok && host != "" {
		connectionOpts.Host = host
	}
	if port, ok := opts["port"].(int); ok && port > 0 {
		connectionOpts.Port = port
	}

	return connectWithOptions(ctx, connectionOpts)
}

func init() {
	transport.RegisterTransport("ssh", &sshCreator{})
}

type ConnectOptions struct {
	User    string
	Host    string
	Port    int
	KeyFile string
	Timeout time.Duration
}

func connectWithOptions(ctx context.Context, opts ConnectOptions) (transport.Connection, error) {
	key, err := os.ReadFile(opts.KeyFile)
	if err != nil {
		return nil, util.WrapWithBase(util.ErrConnectionFailed,
			i18n.T("ssh_key_error", map[string]any{"Error": err}), err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, util.WrapWithBase(util.ErrConnectionFailed,
			i18n.T("ssh_key_error", map[string]any{"Error": err}), err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         opts.Timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Use known_hosts file
	}

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	log.Printf("%s", i18n.T("ssh_dialing", map[string]any{
		"Host": opts.Host,
		"Port": opts.Port,
	}))

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, util.WrapWithBase(util.ErrConnectionFailed, i18n.T("ssh_connect_failed", map[string]any{"Address": addr}), err)
	}

	log.Println(i18n.T("ssh_connection_established", nil))

	cleanup := func(closables ...io.Closer) {
		for _, c := range closables {
			if c != nil {
				_ = c.Close()
			}
		}
	}

	session, err := client.NewSession()
	if err != nil {
		cleanup(client)
		return nil, util.WrapWithBase(util.ErrSessionFailed, i18n.T("ssh_session_failed", nil), err)
	}

	if err = session.RequestSubsystem(Subsystem); err != nil {
		cleanup(session, client)
		return nil, util.WrapWithBase(util.ErrSubsystemFailed,
			i18n.T("ssh_subsystem_failed", map[string]any{"Subsystem": Subsystem}), err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		cleanup(session, client)
		return nil, util.WrapWithBase(util.ErrSubsystemFailed, i18n.T("ssh_stdin_failed", nil), err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		cleanup(session, client)
		return nil, util.WrapWithBase(util.ErrSubsystemFailed, i18n.T("ssh_stdout_failed", nil), err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		cleanup(session, client)
		return nil, util.WrapWithBase(util.ErrSubsystemFailed, i18n.T("ssh_stderr_failed", nil), err)
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

	// Close resources in reverse order they were opened
	if c.stdin != nil {
		_ = c.stdin.Close()
		c.stdin = nil
	}

	if c.session != nil {
		_ = c.session.Close()
		c.session = nil
	}

	if c.client != nil {
		_ = c.client.Close()
		c.client = nil
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

func (c *Conn) SendPayload(payload any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin == nil {
		return fmt.Errorf("%s", i18n.T("connection_closed_payload", nil))
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	data = append(data, '\n')
	_, err = c.stdin.Write(data)

	return err
}
