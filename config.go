package apns

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"regexp"
	"strings"
	"time"
)

// Config описывает конфигурацию для соединения с APNS.
type Config struct {
	BundleId    string // идентификатор приложения
	Sandbox     bool   // флаг отладочного режима
	Certificate tls.Certificate
}

// LoadConfig загружает и возвращает конфигурацию для APNS из JSON-файла.
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile("config.json")
	if err != nil {
		return nil, err
	}
	config := new(Config)
	if err = json.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

// Client возвращает новый инициализированный Client на базе данной конфигурации.
func (config *Config) Connect() (*Conn, error) {
	return Connect(config)
}

// Feedback соединяется с APNS Feedback сервером и возвращает информацию, полученную от него.
func (config *Config) Feedback() ([]*FeedbackResponse, error) {
	return Feedback(config)
}

// Dial устанавливает соединение с APNS-сервисом и возвращает его.
func (config *Config) Dial(addr string) (*tls.Conn, error) {
	serverName, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	tslConfig := &tls.Config{
		Certificates: []tls.Certificate{config.Certificate},
		ServerName:   serverName,
	}
	dialer := &net.Dialer{
		Timeout: time.Duration(5) * time.Second,
	}
	return tls.DialWithDialer(dialer, "tcp", addr, tslConfig)
}

// UnmarshalJSON позволяет читать данную конфигурацию из JSON.
func (config *Config) UnmarshalJSON(data []byte) error {
	dataJSON := new(ConfigJSON)
	if err := json.Unmarshal(data, dataJSON); err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(bytes.Join(dataJSON.Certificate, []byte{'\n'}), dataJSON.PrivateKey)
	if err != nil {
		return err
	}
	*config = Config{
		BundleId:    dataJSON.BundleId,
		Sandbox:     dataJSON.Sandbox,
		Certificate: cert,
	}
	return nil
}

// ConfigJSON описывает структуру конфигурации в формате JSON.
type ConfigJSON struct {
	Type        string   `json:"type"`
	BundleId    string   `json:"bundleId"`
	Sandbox     bool     `json:"sandbox,omitempty"`
	Certificate [][]byte `json:"certificate"`
	PrivateKey  []byte   `json:"privateKey"`
}

// CreateConfig создает описание конфигурации, загружая сертификаты из указанных файлов.
func CreateConfig(bundleId, certFile, keyFile string, sandbox bool) (*ConfigJSON, error) {
	certPEMBlock, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	if bundleId == "" {
		bundlerIds := regexp.MustCompile(`subject=\/UID=([\w\.\-]{3,})\/`).FindStringSubmatch(string(certPEMBlock))
		if len(bundlerIds) > 1 {
			bundleId = bundlerIds[1]
		}
	}

	keyPEMBlock, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	if _, err = tls.X509KeyPair(certPEMBlock, keyPEMBlock); err != nil {
		return nil, err
	}

	cert := make([][]byte, 0, 2)
	var certDERBlock *pem.Block
	for {
		certDERBlock, certPEMBlock = pem.Decode(certPEMBlock)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert = append(cert, pem.EncodeToMemory(certDERBlock))
		}
	}
	if len(cert) == 0 {
		return nil, errors.New("No certificates find")
	}

	var keyDERBlock *pem.Block
	for {
		keyDERBlock, keyPEMBlock = pem.Decode(keyPEMBlock)
		if keyDERBlock == nil {
			return nil, errors.New("failed to parse key PEM data")
		}
		if keyDERBlock.Type == "PRIVATE KEY" || strings.HasSuffix(keyDERBlock.Type, " PRIVATE KEY") {
			break
		}
	}

	config := &ConfigJSON{
		Type:        "apns",
		BundleId:    bundleId,
		Sandbox:     sandbox,
		Certificate: cert,
		PrivateKey:  pem.EncodeToMemory(keyDERBlock),
	}
	return config, nil
}

// printTLSConnectionState выводит в лог информацию о TLS-соединении.
func printTLSConnectionState(conn *tls.Conn) {
	state := conn.ConnectionState()
	log.Println("Connection state:",
		"\n------------------------------------------------------------",
		"\n  Local Address:       ", conn.LocalAddr(),
		"\n  Remote Address:      ", conn.RemoteAddr(),
		"\n  TLS version:         ", state.Version,
		"\n  Handshake Complete:  ", state.HandshakeComplete,
		"\n  Did Resume:          ", state.DidResume,
		"\n  Cipher Suite:        ", state.CipherSuite,
		"\n------------------------------------------------------------")
}
