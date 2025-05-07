package ssh

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"strconv"
	"sync"

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/internal/transport"
	"erlang-solutions.com/cortex_agent/internal/util"
	"golang.org/x/crypto/ssh"
)

const (
	Subsystem      = "cortex"
	ProxySubsystem = "proxy"
)

var _ transport.Connection = (*Conn)(nil)

type Conn struct {
	client       *ssh.Client
	session      *ssh.Session
	stdin        io.WriteCloser
	stdout       io.Reader
	stderr       io.Reader
	proxySession *ssh.Session
	proxyInput   io.WriteCloser
	proxyOutput  io.Reader
	mu           sync.Mutex
}

type sshCreator struct{}

func (c *sshCreator) CreateConnection(ctx context.Context, config config.Config, opts transport.ConnectionOptions) (transport.Connection, error) {
	connectionOpts := transport.ConnectionOptions{
		Protocol: "ssh",
		Host:     config.Connection.Host,
		Port:     config.Connection.Port,
		User:     config.Connection.User,
		KeyFile:  config.Connection.KeyFile,
		Timeout:  config.Connection.Timeout,
		Tunnel:   config.Connection.Tunnel,
	}

	// Override with options if provided
	if opts.Host != "" {
		connectionOpts.Host = opts.Host
	}
	if opts.Port > 0 {
		connectionOpts.Port = opts.Port
	}
	if opts.User != "" {
		connectionOpts.User = opts.User
	}
	if opts.KeyFile != "" {
		connectionOpts.KeyFile = opts.KeyFile
	}
	if opts.Timeout > 0 {
		connectionOpts.Timeout = opts.Timeout
	}

	return connectWithOptions(ctx, connectionOpts)
}

func init() {
	transport.RegisterTransport("ssh", &sshCreator{})
}

func connectWithOptions(ctx context.Context, opts transport.ConnectionOptions) (transport.Connection, error) {
	key, err := os.ReadFile(opts.KeyFile)
	if err != nil {
		return nil, util.NewError(util.ErrTypeConnection,
			i18n.T("ssh_key_error", map[string]any{"Error": err}), err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, util.NewError(util.ErrTypeConnection,
			i18n.T("ssh_key_error", map[string]any{"Error": err}), err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         opts.Timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Use known_hosts file
	}

	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	util.Info(i18n.T("ssh_dialing", map[string]any{
		"Host": opts.Host,
		"Port": opts.Port,
	}))

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, util.NewError(util.ErrTypeConnection,
			i18n.T("ssh_connect_failed", map[string]any{"Address": addr}), err)
	}

	util.Info(i18n.T("ssh_connection_established", map[string]any{}))

	cleanup := func(closables ...io.Closer) {
		for _, c := range closables {
			if c != nil {
				_ = c.Close()
			}
		}
	}

	// Create the main control session (JSON protocol)
	session, err := client.NewSession()
	if err != nil {
		cleanup(client)
		return nil, util.NewError(util.ErrTypeSession, i18n.T("ssh_session_failed", map[string]any{}), err)
	}

	if err = session.RequestSubsystem(Subsystem); err != nil {
		cleanup(session, client)
		return nil, util.NewError(util.ErrTypeSubsystem,
			i18n.T("ssh_subsystem_failed", map[string]any{"Subsystem": Subsystem}), err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		cleanup(session, client)
		return nil, util.NewError(util.ErrTypeSubsystem, i18n.T("ssh_stdin_failed", map[string]any{}), err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		cleanup(session, client)
		return nil, util.NewError(util.ErrTypeSubsystem, i18n.T("ssh_stdout_failed", map[string]any{}), err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		cleanup(session, client)
		return nil, util.NewError(util.ErrTypeSubsystem, i18n.T("ssh_stderr_failed", map[string]any{}), err)
	}

	if !opts.Tunnel {
		return &Conn{
			client:  client,
			session: session,
			stdin:   stdin,
			stdout:  stdout,
			stderr:  stderr,
		}, nil
	}

	util.Info(i18n.T("tunnel_subsystem_creating", map[string]any{}))

	proxySession, err := client.NewSession()
	if err != nil {
		cleanup(session, client)
		return nil, util.NewError(util.ErrTypeSession, i18n.T("tunnel_session_failed", map[string]any{}), err)
	}

	if err = proxySession.RequestSubsystem(ProxySubsystem); err != nil {
		cleanup(proxySession, session, client)
		return nil, util.NewError(util.ErrTypeSubsystem,
			i18n.T("ssh_subsystem_failed", map[string]any{"Subsystem": ProxySubsystem}), err)
	}

	proxyInput, err := proxySession.StdinPipe()
	if err != nil {
		cleanup(proxySession, session, client)
		return nil, util.NewError(util.ErrTypeSubsystem, i18n.T("tunnel_stdin_failed", map[string]any{}), err)
	}

	proxyOutput, err := proxySession.StdoutPipe()
	if err != nil {
		cleanup(proxySession, session, client)
		return nil, util.NewError(util.ErrTypeSubsystem, i18n.T("tunnel_stdout_failed", map[string]any{}), err)
	}

	util.Info(i18n.T("tunnel_subsystem_established", map[string]any{}))

	return &Conn{
		client:       client,
		session:      session,
		stdin:        stdin,
		stdout:       stdout,
		stderr:       stderr,
		proxySession: proxySession,
		proxyInput:   proxyInput,
		proxyOutput:  proxyOutput,
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

	if c.proxyInput != nil {
		_ = c.proxyInput.Close()
		c.proxyInput = nil
	}

	if c.session != nil {
		_ = c.session.Close()
		c.session = nil
	}

	if c.proxySession != nil {
		_ = c.proxySession.Close()
		c.proxySession = nil
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
		return util.NewError(util.ErrTypeConnection, i18n.T("connection_closed_payload", map[string]any{}), nil)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return util.NewError(util.ErrTypeConnection, "failed to marshal payload", err)
	}

	data = append(data, '\n')
	_, err = c.stdin.Write(data)

	return err
}

func (c *Conn) CheckHealth(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil || c.session == nil || c.stdin == nil {
		return util.NewError(util.ErrTypeConnection, i18n.T("connection_closed", map[string]any{}), nil)
	}

	testSession, err := c.client.NewSession()
	if err != nil {
		// SSH connection is broken - most likely due to network issues
		return util.NewError(util.ErrTypeConnection,
			i18n.T("connection_health_check_failed", map[string]any{"Error": err}), err)
	}

	if testSession != nil {
		_ = testSession.Close()
	}

	return nil
}

func (c *Conn) BinaryInput() io.WriteCloser {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.proxyInput // Will be nil if tunnel is disabled
}

func (c *Conn) BinaryOutput() io.Reader {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.proxyOutput // Will be nil if tunnel is disabled
}
