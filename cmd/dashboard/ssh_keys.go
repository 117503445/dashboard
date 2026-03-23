package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

const (
	sshKeyDir       = "data/.ssh"
	privateKeyFile  = "id_ed25519"
	publicKeyFile   = "id_ed25519.pub"
	privateKeyPerms = 0o600
	publicKeyPerms  = 0o644
)

// SSHKeyPair 管理 Dashboard 的 SSH 密钥对
type SSHKeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ssh.PublicKey
	Signer     ssh.Signer
	KeyPath    string
	PubKeyPath string
}

// EnsureSSHKeyPair 确保 SSH 密钥对存在，不存在则生成
func EnsureSSHKeyPair() (*SSHKeyPair, error) {
	keyDir := filepath.Clean(sshKeyDir)
	keyPath := filepath.Join(keyDir, privateKeyFile)
	pubKeyPath := filepath.Join(keyDir, publicKeyFile)

	// 确保目录存在
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建密钥目录失败: %w", err)
	}

	// 检查私钥是否已存在
	if _, err := os.Stat(keyPath); err == nil {
		return loadSSHKeyPair(keyPath, pubKeyPath)
	}

	// 生成新的 ed25519 密钥对
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成密钥对失败: %w", err)
	}

	// 序列化私钥
	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("序列化私钥失败: %w", err)
	}
	privKeyPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privKeyBytes,
	}
	privKeyData := pem.EncodeToMemory(privKeyPEM)

	// 写入私钥
	if err := os.WriteFile(keyPath, privKeyData, privateKeyPerms); err != nil {
		return nil, fmt.Errorf("写入私钥失败: %w", err)
	}

	// 生成公钥 SSH 格式
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("生成公钥失败: %w", err)
	}
	pubKeyData := ssh.MarshalAuthorizedKey(sshPubKey)

	// 写入公钥
	if err := os.WriteFile(pubKeyPath, pubKeyData, publicKeyPerms); err != nil {
		return nil, fmt.Errorf("写入公钥失败: %w", err)
	}

	// 创建 signer
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("创建 signer 失败: %w", err)
	}

	return &SSHKeyPair{
		PrivateKey: privKey,
		PublicKey:  sshPubKey,
		Signer:     signer,
		KeyPath:    keyPath,
		PubKeyPath: pubKeyPath,
	}, nil
}

func loadSSHKeyPair(keyPath, pubKeyPath string) (*SSHKeyPair, error) {
	// 读取私钥
	privKeyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("读取私钥失败: %w", err)
	}

	// 解析私钥
	block, _ := pem.Decode(privKeyData)
	if block == nil {
		return nil, fmt.Errorf("解析私钥 PEM 失败")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	privKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("私钥不是 ed25519 类型")
	}

	// 创建 signer
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("创建 signer 失败: %w", err)
	}

	// 读取公钥
	pubKeyData, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, fmt.Errorf("读取公钥失败: %w", err)
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(pubKeyData)
	if err != nil {
		return nil, fmt.Errorf("解析公钥失败: %w", err)
	}

	return &SSHKeyPair{
		PrivateKey: privKey,
		PublicKey:  pubKey,
		Signer:     signer,
		KeyPath:    keyPath,
		PubKeyPath: pubKeyPath,
	}, nil
}

// PublicKeyString 返回 OpenSSH 格式的公钥字符串
func (k *SSHKeyPair) PublicKeyString() string {
	return string(ssh.MarshalAuthorizedKey(k.PublicKey))
}