package apns

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Notification описывает формат уведомления.
type Notification struct {
	// Содержимое уведомления (не может быть пустым)
	Payload map[string]interface{} `json:"payload"`
	// Время, до которого сообщение является актуальным (должно быть будущее)
	Expiration time.Time `json:"expiration,omitempty"`
	// Приоритет (может быть 0, 5 или 8)
	Priority uint8 `json:"priority,omitempty"`
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
func (ntf *Notification) convert() (*notification, error) {
	if ntf.Payload == nil || len(ntf.Payload) == 0 {
		return nil, ErrPayloadEmpty
	}
	payload, err := json.Marshal(ntf.Payload)
	if err != nil {
		return nil, err
	}
	if len(payload) > MaxPayloadSize { // проверяем, что сообщение допустимого размера
		return nil, ErrPayloadTooLarge
	}
	var expiration uint32
	if !ntf.Expiration.IsZero() {
		if ntf.Expiration.Before(time.Now()) {
			return nil, ErrNotificationExpired
		}
		expiration = uint32(ntf.Expiration.Unix())
	}
	var priority uint8
	if ntf.Priority == 5 || ntf.Priority == 10 {
		priority = ntf.Priority
	}
	var notification = &notification{
		Payload:    payload,
		Expiration: expiration,
		Priority:   priority,
	}
	return notification, nil
}

// notification описывает внутреннее, подготовленное к отправке, представление
// сообщения, используемое внутри приложения.
type notification struct {
	Id         uint32    // уникальный идентификатор уведомления
	Token      []byte    // идентификатор устройства, которому это адресовано
	Payload    []byte    // содержимое уведомления в бинарном виде
	Expiration uint32    // дата и время, после которого сообщение считается не актуальным
	Priority   uint8     // приоритет сообщения: 0, 5 или 8
	Sended     time.Time // время, когда сообщение отправлено на сервер
}

// Len возвращает размер сообщения в байтах, с учетом заголовка
func (ntf *notification) Len() int {
	// 1+4 - заголовок
	// 1+2+32  - токен
	// 1+2+len(payload) - тело сообщения
	var length = 5 + 3 + len(ntf.Token) + 3 + len(ntf.Payload)
	// 1+2+4 - идентификатор сообщения (если есть)
	if ntf.Id != 0 {
		length += 7
	}
	// 1+2+4 - срок окончания актуальности (если есть)
	if ntf.Expiration != 0 {
		length += 7
	}
	// 1+2+1 - приоритет (если есть)
	if ntf.Priority == 5 || ntf.Priority == 10 {
		length += 4
	}
	return length
}

// WriteTo записывает в поток байтовое представление сообщения.
func (ntf *notification) WriteTo(w io.Writer) (n int64, err error) {
	if err = binary.Write(w, binary.BigEndian, uint8(2)); err != nil {
		return
	}
	n += 1
	// не нужно учитывать размер самого заголовка - отнимаем его размер - 5
	if err = binary.Write(w, binary.BigEndian, int32(ntf.Len()-5)); err != nil {
		return
	}
	n += 4

	// device token
	if err = binary.Write(w, binary.BigEndian, uint8(1)); err != nil {
		return
	}
	n += 1
	if err = binary.Write(w, binary.BigEndian, uint16(len(ntf.Token))); err != nil {
		return
	}
	n += 2
	if err = binary.Write(w, binary.BigEndian, ntf.Token); err != nil {
		return
	}
	n += int64(len(ntf.Token))

	// payload
	if err = binary.Write(w, binary.BigEndian, uint8(2)); err != nil {
		return
	}
	n += 1
	if err = binary.Write(w, binary.BigEndian, uint16(len(ntf.Payload))); err != nil {
		return
	}
	n += 2
	if err = binary.Write(w, binary.BigEndian, ntf.Payload); err != nil {
		return
	}
	n += int64(len(ntf.Payload))
	// - Notification identifier
	if ntf.Id != 0 {
		if err = binary.Write(w, binary.BigEndian, uint8(3)); err != nil {
			return
		}
		n += 1
		if err = binary.Write(w, binary.BigEndian, uint16(4)); err != nil {
			return
		}
		n += 2
		if err = binary.Write(w, binary.BigEndian, ntf.Id); err != nil {
			return
		}
		n += 4
	}
	// Expiration date
	if ntf.Expiration != 0 {
		if err = binary.Write(w, binary.BigEndian, uint8(4)); err != nil {
			return
		}
		n += 1
		if err = binary.Write(w, binary.BigEndian, uint16(4)); err != nil {
			return
		}
		n += 2
		if err = binary.Write(w, binary.BigEndian, ntf.Expiration); err != nil {
			return
		}
		n += 4
	}
	// Priority
	if ntf.Priority == 5 || ntf.Priority == 10 {
		if err = binary.Write(w, binary.BigEndian, uint8(5)); err != nil {
			return
		}
		n += 1
		if err = binary.Write(w, binary.BigEndian, uint16(4)); err != nil {
			return
		}
		n += 2
		if err = binary.Write(w, binary.BigEndian, ntf.Priority); err != nil {
			return
		}
		n += 1
	}
	return
}

// WithToken возвращает копию уведомления для отправки с установленным токеном.
// Идентификатор уведомления и дата создания, если они были установлены, при этом сбрасываются.
// Уведомления, полученные с помощью этой функции, полностью готовы для отправки.
func (ntf *notification) WithToken(token []byte) *notification {
	return &notification{
		Token:      token,
		Payload:    ntf.Payload,
		Expiration: ntf.Expiration,
		Priority:   ntf.Priority,
	}
}

// TokenString возвращает строковое представление токена.
func (ntf *notification) TokenString() string { return hex.EncodeToString(ntf.Token) }

// PayloadMap возвращает словарь свойств, определенных для данного сообщения.
func (ntf *notification) PayloadMap() map[string]interface{} {
	var payload = make(map[string]interface{})
	_ = json.Unmarshal(ntf.Payload, &payload)
	return payload
}

// IsExpired возвращает true, если сообщение устарело.
func (ntf *notification) IsExpired() bool {
	return ntf.Expiration != 0 && ntf.Expiration < uint32(time.Now().Unix())
}

// ExpirationTime возвращает время, до которого сообщение является актуальным. Если время жизни
// не было установлено, то возвращает дату, соответствующую time.Time.IsZero().
func (ntf *notification) ExpirationTime() time.Time { return time.Unix(int64(ntf.Expiration), 0) }

// String возвращает короткое строковое описание сообщения в виде токена и номера
// сообщения. Если сообщение не содержит токен устройства, возвращается строка
// с "untokened message" и номером.
func (ntf *notification) String() string {
	if len(ntf.Token) == 0 {
		return fmt.Sprintf("untokened notification [%d]", ntf.Id)
	}
	return fmt.Sprintf("%s [%d]", ntf.TokenString(), ntf.Id)
}
