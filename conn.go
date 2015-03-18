package apns

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// адреса APNS серверов.
const (
	apnsServer        = "gateway.push.apple.com:2195"
	apnsServerSandbox = "gateway.sandbox.push.apple.com:2195"
	waitTime          = 2 * time.Minute // время закрытия соединения, если не активно
)

var (
	SendMessageCacheLifeTime = 5 * time.Minute // как долго хранятся отправленные сообщения
)

// Conn описывает клиента для отправки push-уведомлений через сервер APNS.
type Conn struct {
	conn        *tls.Conn     // соединение с сервером
	config      *Config       // конфигурация и сертификаты
	host        string        // адрес сервера
	isConnected bool          // флаг активного соединения
	cache       *messageCache // кеш отправленных сообщений
	frameBuffer *frameBuffer
	mu          sync.RWMutex // блокировка работы с соединением
}

// Connect возвращает нового инициализированного клиента на основании указанной конфигурации соединения.
func Connect(config *Config) (*Conn, error) {
	var host string
	if config.Sandbox {
		host = apnsServerSandbox
	} else {
		host = apnsServer
	}
	log.Printf("Connecting to %s ...", host)
	conn, err := config.Dial(host) // соединяемся с сервером
	if err != nil {
		return nil, err
	}
	// устанавливаем время ожидания ответа от сервера
	conn.SetReadDeadline(time.Now().Add(waitTime))
	// log.Println("Connected!")
	printTLSConnectionState(conn)
	connection := &Conn{
		conn:        conn,
		config:      config,
		host:        host,
		isConnected: true,
		cache:       newMessageCache(SendMessageCacheLifeTime),
		frameBuffer: &frameBuffer{ // буфер отправляемых сообщений
			Size:  MaxFrameBuffer,
			Delay: FramingTimeout,
		},
	}
	connection.frameBuffer.Send = connection.send // назначаем функцию для отправки буфера
	go connection.handleReads()                   // запускаем чтение ошибок из соединения
	return connection, nil
}

// Send отправляет сообщение на указанные токены устройств.
func (connection *Conn) Send(msg *Message, tokens ...[]byte) error {
	sendmsg, err := msg.toSendMessage() // конвертируем сообщение для отправки
	if err != nil {
		return err
	}
	for _, token := range tokens {
		if len(token) != 32 {
			continue // игнорируем токены неправильной длинны
		}
		connection.frameBuffer.Add(sendmsg.WithToken(token)) // добавляем токе и отправляем
	}
	return nil
}

// handleReads ожидает получения ошибки из открытого соединения с сервером.
func (connection *Conn) handleReads() {
	header := make([]byte, 6) // читаем заголовок сообщения
	_, err := connection.conn.Read(header)
	connection.conn.Close() // после получения ошибки соединение закрывается
	connection.isConnected = false
	if err == nil {
		err = parseAPNSError(header)
	}
	switch err.(type) { // обрабатываем ошибки в зависимости от их типа
	case net.Error: // сетевая ошибка
		err := err.(net.Error)
		if err.Timeout() {
			log.Println("Timeout error, not doing auto reconnect")
			return // не осуществляем подключения
		}
		log.Println("Network Error:", err)
	case apnsError: // ошибка, вернувшаяся от сервер APNS
		err := err.(apnsError)
		if err.Id > 0 {
			log.Printf("Error in message [%d]: %s", err.Id, apnsErrorMessages[err.Status])
			// находим по идентификатору сообщение в кеше
			// исключаем само сообщение с ошибкой, если оно не понравилось Apple
			if resend := connection.cache.FromId(err.Id, err.Status > 0); len(resend) > 0 {
				// посылаем недоставленные сообщения заново
				go func() {
					log.Printf("Repeat sending %d messages from cache", len(resend))
					for _, msg := range resend {
						connection.frameBuffer.Add(msg)
					}
				}()
			}
			connection.cache.Clear() // очищаем кеш
		} else {
			log.Printf("APNS error: %s", apnsErrorMessages[err.Status])
		}
	default:
		if err == io.EOF {
			log.Println("Connection closed by server")
		} else {
			log.Println("Error:", err)
			log.Printf("Type [%T]: %#v", err, err)
		}
	}
	// переподключаемся...
	if err := connection.reconnect(); err != nil {
		panic("unknown network error")
	}

}

func (connection *Conn) send() { // функция отправки сообщения
	// проверяем соединение: если не установлено, то соединяемся
	if connection.conn == nil || !connection.isConnected {
		if err := connection.reconnect(); err != nil {
			panic("unknown network error")
		}
	}
resend:
	connection.mu.Lock()
	n, err := connection.frameBuffer.WriteTo(connection.conn)
	connection.mu.Unlock()
	if err != nil {
		log.Println("Send error:", err)
		if err := connection.reconnect(); err != nil {
			panic("unknown network error")
		}
		goto resend // повторяем попытку отправки
	}
	// увеличиваем время ожидания ответа после успешной отправки данных
	connection.conn.SetReadDeadline(time.Now().Add(waitTime))
	log.Printf("Sended %3d messages (%d bytes)", connection.frameBuffer.Cache.Len(), n)
	connection.frameBuffer.Cache.MoveTo(connection.cache) // переносим в глобальный кеш
}

// reconnect осуществляет цикл подключений к серверу, пока это не случится или пока
// не возникнет ошибка, которую мы не знаем как обрабатывать.
func (connection *Conn) reconnect() error {
	connection.mu.Lock() // блокируем соединение на чтение
	connection.isConnected = false

	// в любом случае, открытое соединение после ошибки закрывается
	if connection.conn != nil {
		connection.conn.Close()
	}
	// пытаемся подключиться...
	log.Printf("Connecting to server %s ...", connection.host)
	var startDuration = time.Duration(10 * time.Second)
	for {
		conn, err := connection.config.Dial(connection.host)
		switch err.(type) {
		case nil: // соединение установлено
			conn.SetReadDeadline(time.Now().Add(waitTime))
			printTLSConnectionState(conn)
			connection.conn = conn
			connection.isConnected = true
			go connection.handleReads() // запускаем чтение ошибок из соединения
			connection.mu.Unlock()
			return nil
		case net.Error: // сетевая ошибка
			err := err.(net.Error)
			log.Println("Error connecting to APNS:", err)
		default: // другая ошибка
			if err == io.EOF {
				log.Println("Connection closed by server")
			} else {
				log.Println("Connection error:", err)
				log.Printf("Type [%T]: %#v", err, err)
				// return err // необрабатываемая ошибка
			}
		}
		log.Printf("Waiting %s ...", startDuration.String())
		time.Sleep(startDuration) // добавляем задержку между попытками
		if startDuration < time.Minute*30 {
			startDuration += time.Duration(10 * time.Second) // увеличиваем задержку
		}
	}
	connection.mu.Unlock()
	return nil
}
