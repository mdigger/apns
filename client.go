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
	ServerApns        = "gateway.push.apple.com:2195"
	ServerApnsSandbox = "gateway.sandbox.push.apple.com:2195"
)

var (
	// Время задержки между переподсоединениями. После каждой ошибки соединения
	// время задержки увеличивается на эту величину, пока не достигнет максимального
	// времени в 30 минут. После это уже расти не будет.
	DurationReconnect = time.Duration(10 * time.Second)
	// время задержки отправки сообщений
	DurationSend = 100 * time.Millisecond
)

type Client struct {
	conn        *tls.Conn          // соединение с сервером
	config      *Config            // конфигурация и сертификаты
	host        string             // адрес сервера
	queue       *notificationQueue // список уведомлений для отправки
	timer       *time.Timer        // таймер задержки отправки
	Delay       time.Duration      // время задержки отправки сообщений
	isConnected bool               // флаг установленного соединения
	mu          sync.Mutex
}

func NewClient(config *Config) *Client {
	var host string
	if config.Sandbox {
		host = ServerApnsSandbox
	} else {
		host = ServerApns
	}
	client := &Client{
		config: config,
		host:   host,
		queue:  newNotificationQueue(),
		Delay:  DurationSend,
	}
	return client
}

// Send отправляет сообщение на указанные токены устройств.
func (client *Client) Send(ntf *Notification, tokens ...[]byte) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.timer != nil {
		client.timer.Stop() // останавливаем таймер, чтобы, не дай бог, не сработал параллельно
	}
	// добавляем сообщение в очередь на отправку
	if err := client.queue.AddNotification(ntf, tokens...); err != nil {
		return err
	}
	// разбираемся с отправкой
	if client.Delay == 0 { // если задержка не установлена, то отправляем сразу
		go client.send()
		return nil
	}
	// устанавливаем или сдвигаем, если уже был установлен, таймер с задержкой отправки
	if client.timer == nil {
		client.timer = time.AfterFunc(client.Delay, client.send)
	} else {
		client.timer.Reset(client.Delay)
	}
	return nil
}

// send непосредственно осуществляет отправку уведомлений на сервер.
func (client *Client) send() {
	defer un(trace("[send]")) // DEBUG
	if !client.queue.IsHasToSend() {
		return // выходим, если нечего отправлять
	}
	for { // делаем это пока не отправим...
		// проверяем соединение: если не установлено, то соединяемся
		if client.conn == nil || !client.isConnected {
			if err := client.Connect(); err != nil {
				panic("unknown network error")
			}
		}
		// отправляем сообщения на сервер
		n, err := client.queue.WriteTo(client.conn)
		if n > 0 {
			log.Printf("Total sended %d bytes", n)
		}
		if err == nil {
			// увеличиваем время ожидания ответа после успешной отправки данных
			client.conn.SetReadDeadline(time.Now().Add(TiemoutRead))
			break // задача выполнена - выходим
		}
		log.Println("Send error:", err)
		client.isConnected = false
	}
}

// Connect устанавливает новое соединение с сервером. Если предыдущее соединение
// при этом было открыто, то оно автоматически закрывается. В случае ошибки установки
// соединения, этот процесс повторяется до бесконечности с постоянно увеличивающимся
// интервалом между попытками.
func (client *Client) Connect() error {
	client.isConnected = false
	if client.conn != nil {
		if err := client.conn.Close(); err != nil {
			log.Println("Close error:", err)
			// return err
		}
	}
	var startDuration = DurationReconnect
	for {
		log.Println("Connecting to server", client.host)
		conn, err := client.config.Dial(client.host)
		switch err.(type) {
		case nil: // соединение установлено
			tlsConnectionStateString(conn)
			client.conn = conn
			client.isConnected = true
			var wg sync.WaitGroup
			wg.Add(1)
			go client.handleReads(&wg) // запускаем чтение ошибок из соединения
			wg.Wait()                  // ждем окончания запуска сервиса
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

// handleReads читает из открытого соединения и ждет получения информации об ошибке.
// После этого автоматически закрывает текущее соединение и запускает процесс установки
// нового соединения, кроме случаев, когда соединение закрыто из-за долгой неактивности.
//
// Если в ответе от сервера содержится информация об идентификаторе ошибочного сообщения,
// то все сообщения, отосланные после него будут заново автоматически отосланы.
func (client *Client) handleReads(wg *sync.WaitGroup) {
	defer un(trace("[handleReads]")) // DEBUG
	wg.Done()                        // сигнализируем о запуске
	var header = make([]byte, 6)     // читаем заголовок сообщения
	_, err := client.conn.Read(header)
	client.conn.Close() // после получения ошибки соединение закрывается
	client.isConnected = false
	if err == nil {
		err = parseAPNSError(header)
	}
	switch err.(type) { // обрабатываем ошибки в зависимости от их типа
	case net.Error: // сетевая ошибка
		var err = err.(net.Error)
		if err.Timeout() {
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
		client.queue.ResendFromId(err.Id, err.Status > 0)
	default:
		if err == io.EOF {
			log.Println("Connection closed by server")
			break
		}
		log.Println("Error:", err)
		log.Printf("Type [%T]: %+v", err, err) // DEBUG
	}
	// переподключаемся...
	if err = client.Connect(); err != nil {
		panic("unknown network error")
	}
}
