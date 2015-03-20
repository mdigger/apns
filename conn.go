package apns

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Conn описывает соединение с APNS-сервером. Соединение отслеживает все ошибки,
// которые могут возвращаться сервером, а так же умеет автоматически переподключаться
// к серверу в случае разрыва соединения.
type Conn struct {
	*tls.Conn           // соединение с сервером
	isConnected bool    // флаг установленного соединения
	client      *Client // клиент соединения
	mu          sync.Mutex
}

// NewConn возвращает соединение, не устанавливая непосредственно соединения с сервером.
func NewConn(client *Client) *Conn {
	return &Conn{client: client}
}

// Dial осуществляет соединение с APNS-сервером и возвращает его. Если соединение
// установить не получилось, то возвращается ошибка.
func Dial(client *Client) (*Conn, error) {
	log.Println("Connecting to server", client.host)
	tlsConn, err := client.config.Dial(client.host)
	if err != nil {
		return nil, err
	}
	log.Print(tlsConnectionStateString(tlsConn))
	var conn = &Conn{
		Conn:        tlsConn,
		isConnected: true,
		client:      client,
	}
	go conn.handleReads() // запускаем чтение ошибок из соединения
	return conn, nil
}

// handleReads читает из открытого соединения и ждет получения информации об ошибке.
// После этого автоматически закрывает текущее соединение и запускает процесс установки
// нового соединения, кроме случаев, когда соединение закрыто из-за долгой неактивности.
//
// Если в ответе от сервера содержится информация об идентификаторе ошибочного сообщения,
// то все сообщения, отосланные после него будут заново автоматически отосланы.
func (conn *Conn) handleReads() {
	// defer un(trace("[handleReads]")) // DEBUG
	var header = make([]byte, 6) // читаем заголовок сообщения
	_, err := conn.Read(header)
	if err == nil {
		err = parseAPNSError(header) // разбираем сообщение и конвертируем в описание ошибки
	}
	// обрабатываем ошибки в зависимости от их типа
	switch err.(type) {
	case net.Error: // сетевая ошибка
		var err = err.(net.Error)
		if err.Timeout() {
			conn.mu.Lock()
			conn.isConnected = false
			conn.mu.Unlock()
			log.Println("Timeout error, not doing auto reconnect")
			return // не осуществляем подключения
		}
		log.Println("Network Error:", err)
	case apnsError: // ошибка, вернувшаяся от сервер APNS
		var err = err.(apnsError)
		if err.Id == 0 {
			log.Printf("APNS error: %s", apnsErrorMessages[err.Status])
			break
		}
		log.Printf("Error in message [%d]: %s", err.Id, apnsErrorMessages[err.Status])
		// послать все сообщения после ошибочного заново
		conn.mu.Lock()
		conn.client.queue.ResendFromId(err.Id, err.Status > 0)
		conn.mu.Unlock()
	default:
		if err == io.EOF {
			log.Println("Connection closed by server")
			break
		}
		log.Println("Error:", err)
		log.Printf("Type [%T]: %+v", err, err) // DEBUG
	}
	// снова подключаемся к серверу
	if err = conn.Connect(); err != nil {
		panic("unknown network error")
	}
}

// Connect устанавливает новое соединение с сервером. Если предыдущее соединение
// при этом было открыто, то оно автоматически закрывается. В случае ошибки установки
// соединения, этот процесс повторяется до бесконечности с постоянно увеличивающимся
// интервалом между попытками.
func (conn *Conn) Connect() error {
	conn.mu.Lock()
	if conn.Conn != nil {
		conn.Conn.Close()
	}
	conn.isConnected = false
	conn.mu.Unlock()
	var startDuration = DurationReconnect
	for {
		log.Println("Connecting to server", conn.client.host)
		tlsConn, err := conn.client.config.Dial(conn.client.host)
		switch err.(type) {
		case nil: // соединение установлено
			log.Print(tlsConnectionStateString(tlsConn))
			conn.mu.Lock()
			conn.Conn = tlsConn
			conn.isConnected = true
			conn.mu.Unlock()
			go conn.handleReads() // запускаем чтение ошибок из соединения
			return nil
		case net.Error: // сетевая ошибка
			err := err.(net.Error)
			log.Println("Error connecting to APNS:", err)
		default: // другая ошибка
			if err == io.EOF {
				log.Println("Connection closed by server")
			} else {
				log.Println("Connection error:", err)
				log.Printf("Type [%T]: %#v", err, err) // DEBUG
				// return err // необрабатываемая ошибка
			}
		}
		log.Printf("Waiting %s ...", startDuration.String())
		time.Sleep(startDuration) // добавляем задержку между попытками
		if startDuration < time.Minute*30 {
			startDuration += DurationReconnect // увеличиваем задержку
		}
	}
}
