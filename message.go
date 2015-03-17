package apns

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrPayloadEmpty    = errors.New("payload is empty")
	ErrPayloadTooLarge = errors.New("payload is too large")
)

const maxPayloadSize = 2048 // максимально допустимая длинна для payload

// Message описывает push-сообщение для отправки.
type Message struct {
	Payload    map[string]interface{} // описание сообщения
	Expiration time.Duration          // продолжительность жизни
	Priority   uint8                  // приоритет: 0, 5 или 10
}

// toSendMessage конвертирует представление сообщения в формат отправляемого сообщения.
// В процессе конвертации проверяется, что сообщение не содержит пустого payload и что
// его длинна не превышает 2K. Время жизни сообщения устанавливается исходя из текущего времени.
//
// Обратите внимание, что получаемое таким образом сообщение не содержит токен устройства
// и не может быть отправлено как есть. Перед отправкой воспользуйтесь методом WithToken()
// для получившегося сообщения. Этот метод вернет копию с уже установленным токеном устройства.
// Таким образом, вы можете легко и без существенного увеличения нагрузки отсылать одно
// и тоже сообщение сразу на большое количество устройств.
func (msg *Message) toSendMessage() (*sendMessage, error) {
	if msg.Payload == nil || len(msg.Payload) == 0 {
		return nil, ErrPayloadEmpty
	}
	payload, err := json.Marshal(msg.Payload)
	if err != nil {
		return nil, err
	}
	if len(payload) > maxPayloadSize { // проверяем, что сообщение допустимого размера
		return nil, ErrPayloadTooLarge
	}
	var expiration uint32
	if msg.Expiration > 0 {
		expiration = uint32(time.Now().Add(msg.Expiration).Unix())
	}
	var priority uint8
	if msg.Priority == 5 || msg.Priority == 10 {
		priority = msg.Priority
	}
	sendMessage := &sendMessage{
		payload:    payload,
		expiration: expiration,
		priority:   priority,
	}
	return sendMessage, nil
}

// sendMessage описывает полное сообщение для отправки, включая сгенерированный идентификатор
// сообщения и токен приложения.
//
// Вам не нужно формировать это сообщение самостоятельно: для этого лучше использовать
// конвертацию из формата Message.SendMessage().
type sendMessage struct {
	id         uint32    // идентификатор (автоматически назначается при отправке)
	token      []byte    // токен устройства
	payload    []byte    // содержимое сообщения
	expiration uint32    // время жизни
	priority   uint8     // приоритет
	created    time.Time // дата и время создания
}

// Id возвращает уникальный идентификатор отправляемого сообщения.
func (smsg *sendMessage) Id() uint32 { return smsg.id }

// Token возвращает строковое представление токена.
func (smsg *sendMessage) Token() string { return hex.EncodeToString(smsg.token) }

// Expiration возвращает время, до которого сообщение является актуальным. Если время жизни
// не было установлено, то возвращает дату, соответствующую time.Time.IsZero().
func (smsg *sendMessage) Expiration() time.Time { return time.Unix(int64(smsg.expiration), 0) }

// IsExpired возвращает true, если данное сообщение устарело.
func (smsg *sendMessage) IsExpired() bool {
	if smsg.expiration == 0 {
		return false
	}
	return (time.Now().Unix() <= int64(smsg.expiration))
}

// Payload возвращает словарь свойств, определенных для данного сообщения.
func (smsg *sendMessage) Payload() map[string]interface{} {
	payload := make(map[string]interface{})
	_ = json.Unmarshal(smsg.payload, &payload)
	return payload
}

// Priority возвращает приоритет сообщения, который может быть 0, 5 или 10.
func (smsg *sendMessage) Priority() uint8 {
	if smsg.priority == 5 || smsg.priority == 10 {
		return smsg.priority
	}
	return 0
}

// String возвращает короткое строковое описание сообщения в виде токена и номера
// сообщения. Если сообщение не содержит токен устройства, возвращается строка
// с "untokened message" и номером.
func (smsg *sendMessage) String() string {
	if len(smsg.token) == 0 {
		return fmt.Sprintf("untokened message [%d]", smsg.id)
	}
	return fmt.Sprintf("%s [%d]", smsg.Token(), smsg.id)
}

// Byte возвращает байтовое представление сообщения.
func (smsg *sendMessage) Byte() []byte {
	buf := getBuffer() // получаем байтовый буфер из пула
	// записываем токен устройства
	binary.Write(buf, binary.BigEndian, uint8(1))
	binary.Write(buf, binary.BigEndian, uint16(32))
	binary.Write(buf, binary.BigEndian, smsg.token)
	// записываем тело сообщение
	binary.Write(buf, binary.BigEndian, uint8(2))
	binary.Write(buf, binary.BigEndian, uint16(len(smsg.payload)))
	binary.Write(buf, binary.BigEndian, smsg.payload)
	// записываем идентификатор сообщения, если он есть
	if smsg.id > 0 {
		binary.Write(buf, binary.BigEndian, uint8(3))
		binary.Write(buf, binary.BigEndian, uint16(4))
		binary.Write(buf, binary.BigEndian, smsg.id)
	}
	// время жизни сообщения, если установлено
	if smsg.expiration != 0 {
		binary.Write(buf, binary.BigEndian, uint8(4))
		binary.Write(buf, binary.BigEndian, uint16(4))
		binary.Write(buf, binary.BigEndian, smsg.expiration)
	}
	// приоритет сообщения, если корректно установлен
	if smsg.priority == 5 || smsg.priority == 10 {
		binary.Write(buf, binary.BigEndian, uint8(5))
		binary.Write(buf, binary.BigEndian, uint16(4))
		binary.Write(buf, binary.BigEndian, smsg.priority)
	}
	// формируем окончательное сообщение
	result := buf.Bytes()
	putBuffer(buf) // возвращаем буфер в пул
	return result
}

// WithToken возвращает копию сообщения для отправки с установленным токеном.
// Идентификатор сообщения, если он был установлен, при этом сбрасывается.
// Сообщения, полученные с помощью этой функции, полностью готовы для отправки.
func (smsg *sendMessage) WithToken(token []byte) *sendMessage {
	return &sendMessage{
		token:      token,
		payload:    smsg.payload,
		expiration: smsg.expiration,
		priority:   smsg.priority,
		created:    time.Now(),
	}
}
