package apns

import (
	"errors"
	"time"
)

// Адреса APNS и Feedback серверов.
const (
	ServerApns            = "gateway.push.apple.com:2195"
	ServerApnsSandbox     = "gateway.sandbox.push.apple.com:2195"
	ServerFeedback        = "feedback.push.apple.com:2196"
	ServerFeedbackSandbox = "feedback.sandbox.push.apple.com:2196"
)

// Используемые сервисом времена задержек и ожиданий.
var (
	// TimeoutConnect указывает время ожидания ответа от сервера при соединении.
	TimeoutConnect = 30 * time.Second
	// DurationReconnect описывает время задержки между переподсоединениями. После каждой ошибки
	// соединения время задержки увеличивается на эту величину, пока не достигнет максимального
	// времени в 30 минут. После это уже расти не будет.
	DurationReconnect = 10 * time.Second
	// TiemoutRead описывает время закрытия соединения, если не активно.
	TiemoutRead = 2 * time.Minute
	// DurationSend описывает время задержки отправки сообщений по умолчанию. Если за это время не
	// добавили ни одного нового сообщения, то буфер отсылается на сервер.
	DurationSend = 100 * time.Millisecond
)

// Используемые по умолчанию значения, для кеширования уведомлений.
var (
	// NotificationCacheSize описывает размер кеша по умолчанию
	NotificationCacheSize = 100
	// MaxFrameBuffer описывает максимальный размер пакета в байтах на отправку
	MaxFrameBuffer = 65535
	// CacheLifeTime описывает как долго хранятся отправленные сообщения
	CacheLifeTime = 5 * time.Minute
)

// MaxPayloadSize описывает максимально допустимую длину для payload уведомления.
var MaxPayloadSize = 2048

// Ошибки, возвращаемые при конвертации уведомлений во внутреннее представление и при добавлении
// уведомлений в очередь на отправку.
var (
	ErrPayloadEmpty        = errors.New("payload is empty")
	ErrPayloadTooLarge     = errors.New("payload is too large")
	ErrNotificationExpired = errors.New("notification expired")
)

// Ошибка добавления уведомления на отправку для закрытого клиента.
var ErrClientIsClosed = errors.New("client is closed")

// Ошибка разбора конфигурации в пустой указатель.
var ErrConfigNil = errors.New("Config: UnmarshalJSON on nil pointer")
