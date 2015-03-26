package apns

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"
)

// Config описывает конфигурацию для соединения с APNS.
type Config struct {
	BundleID    string          // идентификатор приложения
	Sandbox     bool            // флаг отладочного режима
	Certificate tls.Certificate // сертификаты
	log         *log.Logger     // лог для вывода информации
}

// LoadConfig загружает и возвращает конфигурацию для APNS из JSON-файла. Формат такого файла
// описан в ConfigJSON.
func LoadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config = new(Config)
	if err = json.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

// SetLogger позволяет установить свою систему вывода логов.
func (config *Config) SetLogger(llog *log.Logger) {
	if llog == nil {
		prefix := fmt.Sprintf("[apns:%s] ", config.BundleID)
		config.log = log.New(os.Stderr, prefix, log.LstdFlags)
	} else {
		config.log = llog
	}
}

// Feedback соединяется с APNS Feedback сервером и возвращает информацию, полученную от него.
func (config *Config) Feedback() ([]*FeedbackResponse, error) {
	return Feedback(config)
}

// Connect возвращает инициализированный Client с уже установленным соединением для отправки
// уведомлений. Если соединение установить не удалось, то возвращается ошибка.
func (config *Config) Connect() (*Client, error) {
	var client = NewClient(config)
	var err = client.Connect()
	return client, err
}

// Dial устанавливает защищенное соединение с сервером и возвращает его. Время ожидания ответа
// автоматически устанавливается равной TiemoutRead. При желании, вы можете продлевать это время
// самостоятельно после каждого успешного чтения или записи.
func (config *Config) Dial(addr string) (*tls.Conn, error) {
	serverName, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	var (
		tslConfig = &tls.Config{
			ServerName: serverName,
			Certificates: []tls.Certificate{
				config.Certificate,
			},
		}
		dialer = &net.Dialer{
			Timeout: TimeoutConnect,
		}
	)
	// устанавливаем защищенное соединение с сервером
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tslConfig)
	if err != nil {
		return nil, err
	}
	// устанавливаем время ожидания ответа от сервера
	conn.SetReadDeadline(time.Now().Add(TiemoutRead))
	return conn, nil
}

// UnmarshalJSON позволяет читать данную конфигурацию из JSON. Это исключительно вспомогательная
// вещь для поддержки интерфейса JSON.Unmarshaler.
func (config *Config) UnmarshalJSON(data []byte) error {
	var dataJSON = new(ConfigJSON)
	if err := json.Unmarshal(data, dataJSON); err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(
		bytes.Join(dataJSON.Certificate, []byte{'\n'}), dataJSON.PrivateKey)
	if err != nil {
		return err
	}
	if config == nil {
		return ErrConfigNil
	}
	*config = Config{
		BundleID:    dataJSON.BundleID,
		Sandbox:     dataJSON.Sandbox,
		Certificate: cert,
	}
	prefix := fmt.Sprintf("[apns:%s] ", config.BundleID)
	config.log = log.New(os.Stderr, prefix, log.LstdFlags)
	return nil
}

// ConfigJSON описывает структуру конфигурации в формате JSON.
type ConfigJSON struct {
	// тип соединения: должно быть "apns"
	Type string `json:"type"`
	// идентификатор приложения
	BundleID string `json:"bundleId"`
	// флаг соединения с отладочным сервером
	Sandbox bool `json:"sandbox,omitempty"`
	// сертификаты TLS
	Certificate [][]byte `json:"certificate"`
	// приватный ключ
	PrivateKey []byte `json:"privateKey"`
}

// tlsConnectionStateString выводит в лог информацию о TLS-соединении.
func tlsConnectionStateString(conn *tls.Conn) string {
	var state = conn.ConnectionState()
	return fmt.Sprint("Connection state:",
		"\n------------------------------------------------------------",
		"\n  Local Address:       ", conn.LocalAddr(),
		"\n  Remote Address:      ", conn.RemoteAddr(),
		"\n  TLS version:         ", state.Version,
		"\n  Handshake Complete:  ", state.HandshakeComplete,
		"\n  Did Resume:          ", state.DidResume,
		"\n  Cipher Suite:        ", state.CipherSuite,
		"\n------------------------------------------------------------")
}
