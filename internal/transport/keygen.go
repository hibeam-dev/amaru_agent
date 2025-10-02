package transport

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/hibeam-dev/amaru_agent/internal/i18n"
)

type KeyPair struct {
	PrivateKey []byte
	PublicKey  []byte
	KeyPath    string
}

type KeyGenerator interface {
	GenerateKey(keyPath string) (*KeyPair, error)
}

type Ed25519Generator struct{}

func NewEd25519Generator() KeyGenerator {
	return &Ed25519Generator{}
}

func (g *Ed25519Generator) GenerateKey(keyPath string) (*KeyPair, error) {
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return nil, fmt.Errorf(i18n.T("keygen_key_dir_create_error", map[string]interface{}{"Error": err}), err)
	}

	if err := g.backupExistingKeys(keyPath); err != nil {
		return nil, fmt.Errorf(i18n.T("keygen_backup_error", map[string]interface{}{"Error": err}), err)
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("keygen_key_generate_error", map[string]interface{}{"Error": err}), err)
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("keygen_private_key_marshal_error", map[string]interface{}{"Error": err}), err)
	}

	privateKeyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("keygen_ssh_public_key_error", map[string]interface{}{"Error": err}), err)
	}
	publicKeySSH := ssh.MarshalAuthorizedKey(sshPublicKey)

	if err := os.WriteFile(keyPath, privateKeyPem, 0600); err != nil {
		return nil, fmt.Errorf(i18n.T("keygen_private_key_write_error", map[string]interface{}{"Error": err}), err)
	}

	publicKeyPath := keyPath + ".pub"
	if err := os.WriteFile(publicKeyPath, publicKeySSH, 0644); err != nil {
		return nil, fmt.Errorf(i18n.T("keygen_public_key_write_error", map[string]interface{}{"Error": err}), err)
	}

	return &KeyPair{
		PrivateKey: privateKeyPem,
		PublicKey:  publicKeySSH,
		KeyPath:    keyPath,
	}, nil
}

func (g *Ed25519Generator) backupExistingKeys(keyPath string) error {
	publicKeyPath := keyPath + ".pub"

	_, privateKeyExists := os.Stat(keyPath)
	_, publicKeyExists := os.Stat(publicKeyPath)

	if privateKeyExists != nil && publicKeyExists != nil {
		return nil
	}

	timestamp := time.Now().Format("20060102-150405")
	backupSuffix := ".bak." + timestamp

	if privateKeyExists == nil {
		if err := g.copyFile(keyPath, keyPath+backupSuffix); err != nil {
			return fmt.Errorf(i18n.T("keygen_private_key_backup_error", map[string]interface{}{"Error": err}), err)
		}
	}

	if publicKeyExists == nil {
		if err := g.copyFile(publicKeyPath, publicKeyPath+backupSuffix); err != nil {
			return fmt.Errorf(i18n.T("keygen_public_key_backup_error", map[string]interface{}{"Error": err}), err)
		}
	}

	return nil
}

func (g *Ed25519Generator) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Get original file permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}
