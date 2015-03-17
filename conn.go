package apns

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// адреса APNS серверов.
const (
	apnsServer          = "gateway.push.apple.com:2195"
	apnsServerSandbox   = "gateway.sandbox.push.apple.com:2195"
	sendMessageLifeTime = 5 * time.Minute // как долго храним отправленные сообщения
	waitTime            = 2 * time.Minute // время закрытия соединения, если не активно
	tcpFrameMax         = 65535           // максимальный размер TCP-фрейма в байтах
)

var (
	FramingTimeout = 100 * time.Millisecond // время задержки отправки сообщений
)

// Conn описывает клиента для отправки push-уведомлений через сервер APNS.
type Conn struct {
	conn            *tls.Conn         // соединение с сервером
	config          *Config           // конфигурация и сертификаты
	host            string            // адрес сервера
	sendMessageChan chan *sendMessage // канал передачи сообщений на отправку
	cache           []*sendMessage    // кеш отправленных сообщений
	cacheBuffer     []*sendMessage    // кеш добавленных в буфер на отправку сообщений
	frameBuffer     *bytes.Buffer     // буфер отправляемых сообщений
	frameTimer      *time.Timer       // таймер отправки буфера сообщений
	counter         uint32            // счетчик отправленных сообщений
	mu              sync.Mutex        // блокировка записи в буфер
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
		conn:            conn,
		config:          config,
		host:            host,
		sendMessageChan: make(chan *sendMessage),
		cache:           make([]*sendMessage, 0),
		cacheBuffer:     make([]*sendMessage, 0),
		frameBuffer:     new(bytes.Buffer),
	}
	go connection.handleReads() // запускаем чтение ошибок из соединения
	go connection.waitLoop()    // запускаем сервис чтения и записи сообщений
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
		connection.sendMessageChan <- sendmsg.WithToken(token) // добавляем токе и отправляем
	}
	return nil
}

// reconnect устанавливает соединение с сервером APNS.
// При установке делается бесконечное число попыток с небольшой задержкой, пока соединение
// не будет установлено. Возвращает ошибку только в том случае, если это не ошибка соединения.
func (connection *Conn) reconnect() error {
	log.Printf("Connecting to server %s ...", connection.host)
	startDuration := time.Duration(10 * time.Second)
	for {
		conn, err := connection.config.Dial(connection.host)
		if err == nil { // соединение установлено
			// log.Println("Connected")
			// Apple разрывает соединение, если в нем некоторое время нет активности.
			// Поэтому устанавливаем время активности. Позже, мы будем его продлевать
			// после каждой успешной отправки сообщений.
			conn.SetReadDeadline(time.Now().Add(waitTime))
			printTLSConnectionState(conn)
			connection.mu.Lock()
			connection.conn = conn
			connection.mu.Unlock()
			go connection.handleReads() // запускаем чтение ошибок из соединения
			return nil
		}
		if err, ok := err.(net.Error); ok { // сетевая ошибка
			log.Println("Error connecting to APNS:", err)
			log.Printf("Waiting %s ...", startDuration.String())
			time.Sleep(startDuration) // добавляем задержку между попытками
			if startDuration < time.Minute*30 {
				startDuration += time.Duration(10 * time.Second) // увеличиваем задержку
			}
			continue
		}
		if err == io.EOF {
			log.Println("Connection closed by server")
			log.Printf("Waiting %s ...", startDuration.String())
			time.Sleep(startDuration) // добавляем задержку между попытками
			continue
		}
		// неизвестная и необрабатываемая нами ошибка
		log.Println("Connection error:", err)
		log.Printf("Type [%T]: %#v", err, err)
		return err // не сетевая ошибка
	}
}

// handleReads ожидает получения ошибки из открытого соединения с сервером.
func (connection *Conn) handleReads() {
	header := make([]byte, 6) // читаем заголовок сообщения
	// connection.mu.Lock()
	n, err := connection.conn.Read(header)
	// connection.mu.Unlock()
	switch {
	case err != nil:
		connection.handleError(err)
	case n == 6:
		connection.handleError(parseAPNSError(header))
	default:
		connection.handleError(errors.New("bad apple error size"))
	}
}

// handleError обрабатывает полученные от сервера ошибки.
func (connection *Conn) handleError(err error) {
	connection.conn.Close() // в любом случае закрываем соединение при любой ошибке
	// connection.conn = nil
	switch err.(type) {
	case net.Error:
		err := err.(net.Error)
		if err.Timeout() {
			log.Println("Timeout error, not doing auto reconnect")
			return
		}
		log.Println("Network Error:", err)
	case apnsError:
		err := err.(apnsError)
		if err.Id > 0 {
			log.Printf("Error in message [%d]: %s", err.Id, apnsErrorMessages[err.Status])
			// находим по идентификатору сообщение в кеше
			cache := connection.cache[:]
			var index int
			for i, msg := range cache {
				if msg.id == err.Id {
					index = i
					break
				}
			}
			// исключаем само сообщение с ошибкой, если оно не понравилось Apple
			if err.Status > 0 {
				index = index + 1
			}
			if resend := connection.cache[index:]; len(resend) > 0 {
				// посылаем недоставленные сообщения заново
				go func() {
					log.Printf("Repeat sending %d messages from cache", len(resend))
					for _, msg := range resend {
						connection.sendMessageChan <- msg
					}
				}()
			}
			connection.cache = make([]*sendMessage, 0) // очищаем кеш
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
	// пытаемся подключиться...
	// connection.mu.Lock()
	if err := connection.reconnect(); err != nil {
		panic("Non network error connecting") // ошибка - нет сети совсем
	}
	// connection.mu.Unlock()
}

// waitLoop запускает бесконечный цикл обработки ошибок и отправки сообщений.
func (connection *Conn) waitLoop() {
	var (
		cleanup = time.Tick(sendMessageLifeTime) // время для очистки кеша
	)
	for {
		select {
		case msg := <-connection.sendMessageChan: // новое сообщение на отправку
			if msg.IsExpired() {
				break // пропускаем устаревшее сообщение
			}
			if msg.id == 0 {
				connection.mu.Lock()
				connection.counter++
				msg.id = connection.counter // присваиваем уникальный идентификатор
				connection.mu.Unlock()
			}
			// формируем байтовое представление сообщения и добавляем его в буфер на отправку
			data := msg.Byte()
			if connection.frameBuffer.Len()+len(data) > tcpFrameMax {
				// log.Println("Frame buffer is full")
				connection.sendBuffer()
			}
			// добавляем сообщение в буфер на отправку
			connection.mu.Lock()
			binary.Write(connection.frameBuffer, binary.BigEndian, uint8(2))
			binary.Write(connection.frameBuffer, binary.BigEndian, uint32(len(data)))
			bytes.NewReader(data).WriteTo(connection.frameBuffer)
			connection.cacheBuffer = append(connection.cacheBuffer, msg) // сохраняем в локальный кеш
			// взводим таймер задержки отправки сообщений
			if connection.frameTimer == nil {
				connection.frameTimer = time.AfterFunc(FramingTimeout, connection.sendBuffer)
			} else {
				connection.frameTimer.Reset(FramingTimeout)
			}
			connection.mu.Unlock()
		case <-cleanup: // время для очистки кеша
			connection.mu.Lock()
			cache := connection.cache
			if len(cache) == 0 {
				break
			}
			// удаляем "устаревшие" сообщения
			live := make([]*sendMessage, 0)
			for _, msg := range cache {
				if time.Since(msg.created) < sendMessageLifeTime {
					live = append(live, msg)
				}
			}
			if count := len(cache) - len(live); count > 0 {
				log.Printf("Deleted %d old messages in cache", count)
				connection.cache = live
			}
			connection.mu.Unlock()
		}
	}
}

// sendBuffer отправляет сообщения, хранящиеся в буфере на отправку.
func (connection *Conn) sendBuffer() {
	connection.mu.Lock()
	connection.frameTimer.Stop()
	if connection.frameBuffer.Len() == 0 {
		connection.mu.Unlock()
		return
	}
	n, err := connection.frameBuffer.WriteTo(connection.conn)
	connection.mu.Unlock()
	if err != nil {
		log.Println("Send error:", err)
		connection.handleError(errors.New("write error"))
		go func(cacheBuffer []*sendMessage) { // отсылаем сообщения еще раз
			log.Printf("Resend %3d last messages", len(cacheBuffer))
			for _, msg := range cacheBuffer {
				connection.sendMessageChan <- msg
			}
		}(connection.cacheBuffer)
		connection.cacheBuffer = make([]*sendMessage, 0)
		return
	}
	log.Printf("Sended %3d messages (%d bytes)", len(connection.cacheBuffer), n)
	// увеличиваем время ожидания ответа после успешной отправки данных
	connection.conn.SetReadDeadline(time.Now().Add(waitTime))
	connection.mu.Lock()
	connection.cache = append(connection.cache, connection.cacheBuffer...) // сохраняем в кеш
	connection.cacheBuffer = make([]*sendMessage, 0)                       // сбрасываем локальный кеш
	connection.mu.Unlock()
}
