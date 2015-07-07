package apns

import (
	"encoding/binary"
	"encoding/hex"
	"io"
	"time"
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
	config.log.Println("Feedback connection")
	// config.log.Print(tlsConnectionStateString(conn))

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
			Timestamp: binary.BigEndian.Uint32(header[0:4]),
			Token:     tokenBuffer,
		}
		result = append(result, response)
	}
}

// FeedbackResponse описывает формат элемента ответа от feedback сервера.
type FeedbackResponse struct {
	Timestamp uint32 // метка времени
	Token     []byte // токен устройства
}

// String возвращает строковое представление токена.
func (fr *FeedbackResponse) String() string { return hex.EncodeToString(fr.Token) }

// Time возвращает время генерации сообщения.
func (fr *FeedbackResponse) Time() time.Time { return time.Unix(int64(fr.Timestamp), 0) }
