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
	"erlang-solutions.com/cortex_agent/pkg/result"
	"golang.org/x/crypto/ssh"
)

func ConnectResult(ctx context.Context, cfg config.Config) result.Result[Connection] {
	keyResult := readKeyFile(cfg.Connection.KeyFile)
	if keyResult.IsErr() {
		return result.Err[Connection](keyResult.Error())
	}
	key := keyResult.Value()

	signerResult := parsePrivateKey(key)
	if signerResult.IsErr() {
		return result.Err[Connection](signerResult.Error())
	}
	signer := signerResult.Value()

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

	clientResult := dialSSH("tcp", addr, sshConfig)
	if clientResult.IsErr() {
		return result.Err[Connection](clientResult.Error())
	}
	client := clientResult.Value()

	log.Println(i18n.T("ssh_connection_established", nil))
	log.Println(i18n.T("ssh_server_auth", nil))

	sessionResult := createSession(client)
	if sessionResult.IsErr() {
		_ = client.Close()
		return result.Err[Connection](sessionResult.Error())
	}
	session := sessionResult.Value()

	subsystemResult := requestSubsystem(session, Subsystem)
	if subsystemResult.IsErr() {
		_ = session.Close()
		_ = client.Close()
		return result.Err[Connection](subsystemResult.Error())
	}

	pipeResult := getPipes(session)
	if pipeResult.IsErr() {
		_ = session.Close()
		_ = client.Close()
		return result.Err[Connection](pipeResult.Error())
	}
	pipes := pipeResult.Value()

	conn := &Conn{
		client:  client,
		session: session,
		stdin:   pipes.stdin,
		stdout:  pipes.stdout,
		stderr:  pipes.stderr,
	}

	return result.Ok[Connection](conn)
}

type SessionPipes struct {
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

func readKeyFile(path string) result.Result[[]byte] {
	key, err := os.ReadFile(path)
	if err != nil {
		wrappedErr := errors.WrapWithBase(errors.ErrSSHConnect,
			i18n.T("ssh_key_error", map[string]interface{}{"Error": err}), err)
		return result.Err[[]byte](wrappedErr)
	}
	return result.Ok(key)
}

func parsePrivateKey(key []byte) result.Result[ssh.Signer] {
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		wrappedErr := errors.WrapWithBase(errors.ErrSSHConnect,
			i18n.T("ssh_key_error", map[string]interface{}{"Error": err}), err)
		return result.Err[ssh.Signer](wrappedErr)
	}
	return result.Ok(signer)
}

func dialSSH(network, addr string, config *ssh.ClientConfig) result.Result[*ssh.Client] {
	client, err := ssh.Dial(network, addr, config)
	if err != nil {
		wrappedErr := errors.WrapWithBase(errors.ErrSSHConnect,
			i18n.T("ssh_connect_failed", map[string]interface{}{"Address": addr}), err)
		return result.Err[*ssh.Client](wrappedErr)
	}
	return result.Ok(client)
}

func createSession(client *ssh.Client) result.Result[*ssh.Session] {
	session, err := client.NewSession()
	if err != nil {
		wrappedErr := errors.WrapWithBase(errors.ErrSSHSession,
			i18n.T("ssh_session_failed", nil), err)
		return result.Err[*ssh.Session](wrappedErr)
	}
	return result.Ok(session)
}

func requestSubsystem(session *ssh.Session, subsystem string) result.Result[struct{}] {
	err := session.RequestSubsystem(subsystem)
	if err != nil {
		wrappedErr := errors.WrapWithBase(errors.ErrSSHSubsystem,
			i18n.T("ssh_subsystem_failed", map[string]interface{}{"Subsystem": subsystem}), err)
		return result.Err[struct{}](wrappedErr)
	}
	return result.Ok(struct{}{})
}

func getPipes(session *ssh.Session) result.Result[SessionPipes] {
	stdin, err := session.StdinPipe()
	if err != nil {
		wrappedErr := errors.WrapWithBase(errors.ErrSSHSubsystem,
			i18n.T("ssh_stdin_failed", nil), err)
		return result.Err[SessionPipes](wrappedErr)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		wrappedErr := errors.WrapWithBase(errors.ErrSSHSubsystem,
			i18n.T("ssh_stdout_failed", nil), err)
		return result.Err[SessionPipes](wrappedErr)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		wrappedErr := errors.WrapWithBase(errors.ErrSSHSubsystem,
			i18n.T("ssh_stderr_failed", nil), err)
		return result.Err[SessionPipes](wrappedErr)
	}

	return result.Ok(SessionPipes{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	})
}

func SendPayloadResult[T any](conn Connection, payload T) result.Result[struct{}] {
	if err := conn.SendPayload(payload); err != nil {
		return result.Err[struct{}](fmt.Errorf("failed to send payload: %w", err))
	}
	return result.Ok(struct{}{})
}

func ParseJSONResult[T any](data []byte) result.Result[T] {
	var parsed T
	if err := json.Unmarshal(data, &parsed); err != nil {
		return result.Err[T](fmt.Errorf("failed to parse JSON: %w", err))
	}
	return result.Ok(parsed)
}
