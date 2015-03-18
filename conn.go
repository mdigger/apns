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
	errorChan       chan error        // канал передачи ошибок
	mu              sync.RWMutex      // блокировка работы с соединением
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
		errorChan:       make(chan error),
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

// handleReads ожидает получения ошибки из открытого соединения с сервером.
func (connection *Conn) handleReads() {
	header := make([]byte, 6) // читаем заголовок сообщения
	n, err := connection.conn.Read(header)
	switch {
	case err != nil:
		connection.errorChan <- err
	case n == 6:
		connection.errorChan <- parseAPNSError(header)
	default:
		connection.errorChan <- errors.New("bad apple error size")
	}
}

// waitLoop запускает бесконечный цикл обработки ошибок и отправки сообщений.
func (connection *Conn) waitLoop() {
	var (
		cleanup     = time.Tick(sendMessageLifeTime) // время для очистки кеша
		counter     uint32                           // счетчик отправленных сообщений
		frameBuffer = new(bytes.Buffer)              // буфер отправляемых сообщений
		frameTimer  *time.Timer                      // таймер отправки буфера сообщений
		cache       = make([]*sendMessage, 0)        // кеш отправленных сообщений
		cacheBuffer = make([]*sendMessage, 0)        // кеш добавленных в буфер на отправку сообщений
		mu          sync.RWMutex                     // блокировка работы с буфером
		send        = func() {
			frameTimer.Stop() // останавливаем таймер задержки отправки
			// проверяем соединение: если не установлено, то соединяемся
		reconnect:
			connection.mu.Lock() // если идет переустановка соединения, то ждем.
			if connection.conn == nil {
				if err := connection.reconnect(); err != nil {
					panic("unknown network error")
				}
			}
			connection.mu.Unlock()
			connection.mu.RLock()
			mu.Lock()
			n, err := frameBuffer.WriteTo(connection.conn)
			mu.Unlock()
			connection.mu.RUnlock()
			// разблокируем возможность установки соединения
			if err != nil {
				log.Println("Send error:", err)
				goto reconnect // повторяем попытку отправки
			} else {
				log.Printf("Sended %3d messages (%d bytes)", len(cacheBuffer), n)
				// увеличиваем время ожидания ответа после успешной отправки данных
				connection.conn.SetReadDeadline(time.Now().Add(waitTime))
				mu.RLock()
				cache = append(cache, cacheBuffer...) // сохраняем в кеш
				cacheBuffer = make([]*sendMessage, 0) // сбрасываем локальный кеш
				mu.RUnlock()
			}
		} // функция отправки сообщения
	)

	for {
		select {

		// Новое входящее сообщение для отправки
		case msg := <-connection.sendMessageChan: // новое сообщение на отправку
			if msg.IsExpired() {
				break // пропускаем устаревшее сообщение
			}
			if msg.id == 0 {
				counter++
				msg.id = counter // присваиваем уникальный идентификатор
			}
			// формируем байтовое представление сообщения и добавляем его в буфер на отправку
			data := msg.Byte()
			mu.Lock() // блокируем изменение локальных переменных
			length := frameBuffer.Len() + len(data)
			mu.Unlock() // разблокируем изменение локальных переменных
			if length > tcpFrameMax {
				send()
			}
			// добавляем сообщение в буфер на отправку
			mu.RLock() // блокируем чтение локальных беременных из других потоков
			binary.Write(frameBuffer, binary.BigEndian, uint8(2))
			binary.Write(frameBuffer, binary.BigEndian, uint32(len(data)))
			bytes.NewReader(data).WriteTo(frameBuffer)
			cacheBuffer = append(cacheBuffer, msg) // сохраняем в локальный кеш
			// взводим таймер задержки отправки сообщений
			if frameTimer == nil {
				frameTimer = time.AfterFunc(FramingTimeout, send)
			} else {
				frameTimer.Reset(FramingTimeout)
			}
			mu.RUnlock() // разблокируем чтение локальных беременных из других потоков

			// Очистка кеша устаревших отправленных сообщений
		case <-cleanup: // время для очистки кеша
			l := len(cache) - 1
			if l == -1 {
				break
			}
			// удаляем "устаревшие" сообщения
			for i := l; i >= 0; i-- {
				if time.Since(cache[i].created) >= sendMessageLifeTime {
					if i < l {
						log.Printf("Deleted %d old messages in cache", l-i)
						cache = cache[i:]
					}
					break
				}
			}

			// Обработка ошибок
		case err := <-connection.errorChan: // получена ошибка
			switch err.(type) { // обрабатываем ошибки в зависимости от их типа
			case net.Error: // сетевая ошибка
				err := err.(net.Error)
				if err.Timeout() {
					log.Println("Timeout error, not doing auto reconnect")
					continue // не осуществляем подключения
				}
				log.Println("Network Error:", err)
			case apnsError: // ошибка, вернувшаяся от сервер APNS
				err := err.(apnsError)
				if err.Id > 0 {
					log.Printf("Error in message [%d]: %s", err.Id, apnsErrorMessages[err.Status])
					// находим по идентификатору сообщение в кеше
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
					if resend := cache[index:]; len(resend) > 0 {
						// посылаем недоставленные сообщения заново
						go func() {
							log.Printf("Repeat sending %d messages from cache", len(resend))
							for _, msg := range resend {
								connection.sendMessageChan <- msg
							}
						}()
					}
					cache = make([]*sendMessage, 0) // очищаем кеш
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
	}
}

// reconnect осуществляет цикл подключений к серверу, пока это не случится или пока
// не возникнет ошибка, которую мы не знаем как обрабатывать.
func (connection *Conn) reconnect() error {
	connection.mu.RLock() // блокируем соединение на чтение
	defer connection.mu.RUnlock()

	// в любом случае, открытое соединение после ошибки закрывается
	if connection.conn != nil {
		connection.conn.Close()
		connection.conn = nil
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
			go connection.handleReads() // запускаем чтение ошибок из соединения
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
}
