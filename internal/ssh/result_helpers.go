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

	"erlang-solutions.com/cortex_agent/internal/config"
	"erlang-solutions.com/cortex_agent/internal/i18n"
	"erlang-solutions.com/cortex_agent/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func ConnectWithHelpers(ctx context.Context, cfg config.Config) (Connection, error) {
	key, err := readKeyFile(cfg.Connection.KeyFile)
	if err != nil {
		return nil, err
	}

	signer, err := parsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.Connection.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         cfg.Connection.Timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Use known_hosts file
	}

	addr := net.JoinHostPort(cfg.Connection.Host, strconv.Itoa(cfg.Connection.Port))

	msg := i18n.T("ssh_dialing", map[string]interface{}{
		"Host": cfg.Connection.Host,
		"Port": cfg.Connection.Port,
	})
	log.Printf("%s", msg)

	client, err := dialSSH("tcp", addr, sshConfig)
	if err != nil {
		return nil, err
	}

	log.Println(i18n.T("ssh_connection_established", nil))
	log.Println(i18n.T("ssh_server_auth", nil))

	session, err := createSession(client)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	err = requestSubsystem(session, Subsystem)
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, err
	}

	pipes, err := getPipes(session)
	if err != nil {
		_ = session.Close()
		_ = client.Close()
		return nil, err
	}

	conn := &Conn{
		client:  client,
		session: session,
		stdin:   pipes.stdin,
		stdout:  pipes.stdout,
		stderr:  pipes.stderr,
	}

	return conn, nil
}

type SessionPipes struct {
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

func readKeyFile(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrConnectionFailed,
			i18n.T("ssh_key_error", map[string]interface{}{"Error": err}), err)
	}
	return key, nil
}

func parsePrivateKey(key []byte) (ssh.Signer, error) {
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrConnectionFailed,
			i18n.T("ssh_key_error", map[string]interface{}{"Error": err}), err)
	}
	return signer, nil
}

func dialSSH(network, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	client, err := ssh.Dial(network, addr, config)
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrConnectionFailed,
			i18n.T("ssh_connect_failed", map[string]interface{}{"Address": addr}), err)
	}
	return client, nil
}

func createSession(client *ssh.Client) (*ssh.Session, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, errors.WrapWithBase(errors.ErrSessionFailed,
			i18n.T("ssh_session_failed", nil), err)
	}
	return session, nil
}

func requestSubsystem(session *ssh.Session, subsystem string) error {
	err := session.RequestSubsystem(subsystem)
	if err != nil {
		return errors.WrapWithBase(errors.ErrSubsystemFailed,
			i18n.T("ssh_subsystem_failed", map[string]interface{}{"Subsystem": subsystem}), err)
	}
	return nil
}

func getPipes(session *ssh.Session) (SessionPipes, error) {
	stdin, err := session.StdinPipe()
	if err != nil {
		return SessionPipes{}, errors.WrapWithBase(errors.ErrSubsystemFailed,
			i18n.T("ssh_stdin_failed", nil), err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return SessionPipes{}, errors.WrapWithBase(errors.ErrSubsystemFailed,
			i18n.T("ssh_stdout_failed", nil), err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return SessionPipes{}, errors.WrapWithBase(errors.ErrSubsystemFailed,
			i18n.T("ssh_stderr_failed", nil), err)
	}

	return SessionPipes{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}, nil
}

func SendPayload[T any](conn Connection, payload T) error {
	if err := conn.SendPayload(payload); err != nil {
		return fmt.Errorf("failed to send payload: %w", err)
	}
	return nil
}

func ParseJSON[T any](data []byte) (T, error) {
	var parsed T
	if err := json.Unmarshal(data, &parsed); err != nil {
		return parsed, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return parsed, nil
}
