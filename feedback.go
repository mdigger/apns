package apns

import (
	"encoding/binary"
	"encoding/hex"
	"io"
	"time"
)

// адреса feedback серверов.
const (
	ServerFeedback        = "feedback.push.apple.com:2196"
	ServerFeedbackSandbox = "feedback.sandbox.push.apple.com:2196"
)

// Feedback осуществляет соединение с feedback сервером и возвращает список ответов от него.
// После этого соединение автоматически закрывается.
func Feedback(config *Config) ([]*FeedbackResponse, error) {
	var addr string
	if config.Sandbox {
		addr = ServerFeedbackSandbox
	} else {
		addr = ServerFeedback
	}
	conn, err := config.Dial(addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	// log.Print(tlsConnectionStateString(conn))

	var (
		result = make([]*FeedbackResponse, 0)
		header = make([]byte, 6)
	)
	for {
		if _, err = conn.Read(header); err != nil {
			if err == io.EOF {
				err = nil
			}
			return result, err
		}
		var (
			tokenSize   = int(binary.BigEndian.Uint16(header[4:6]))
			tokenBuffer = make([]byte, tokenSize)
		)
		if _, err = conn.Read(tokenBuffer); err != nil {
			if err == io.EOF {
				err = nil
			}
			return result, err
		}
		var response = &FeedbackResponse{
			timestamp: binary.BigEndian.Uint32(header[0:4]),
			Token:     tokenBuffer,
		}
		result = append(result, response)
	}
}

// FeedbackResponse описывает формат элемента ответа от feedback сервера.
type FeedbackResponse struct {
	timestamp uint32
	Token     []byte
}

// String возвращает строковое представление токена.
func (fr *FeedbackResponse) String() string { return hex.EncodeToString(fr.Token) }

// Time возвращает время генерации сообщения.
func (fr *FeedbackResponse) Time() time.Time { return time.Unix(int64(fr.timestamp), 0) }
