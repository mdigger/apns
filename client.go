package apns

import (
	"sync"
	"time"
)

// Client описывает клиента для соединения с APNS и отправки уведомлений.
type Client struct {
	conn      *apnsConn          // соединение с сервером
	config    *Config            // конфигурация и сертификаты
	host      string             // адрес сервера
	queue     *notificationQueue // список уведомлений для отправки
	isSendign bool               // флаг активности отправки
	isClosed  bool               // флаг закрытия клиента
	mu        sync.RWMutex       // блокировка доступа к флагу посылки
}

// NewClient возвращает инициализированный клиент для отправки уведомлений на APNS. Подключения
// к APNS сервису при этом не происходит: оно произойдет автоматически, когда через него попытаются
// отправить первое уведомление.
func NewClient(config *Config) *Client {
	var host string
	if config.Sandbox {
		host = ServerApnsSandbox
	} else {
		host = ServerApns
	}
	var client = &Client{
		config: config,
		host:   host,
		queue:  newNotificationQueue(),
	}
	client.conn = &apnsConn{client: client}
	return client
}

// Connect осуществляет подключение к APNS и возвращает ошибку, если подключение установить
// не удалось.
//
// Обычно не требуется вызывать эту функцию вручную, т.к. подключение к серверу инициализируется
// автоматически при добавлении уведомления в очередь на отправку. Так же, в случае долгого
// не использования сервиса, переподключение к серверу тоже произойдет автоматичеки, когда
// потребуется отправить новые данные.
func (client *Client) Connect() error {
	client.config.log.Println("Connecting to server", client.host)
	tlsConn, err := client.config.Dial(client.host)
	if err != nil {
		return err
	}
	client.config.log.Print(tlsConnectionStateString(tlsConn))
	var conn = &apnsConn{
		Conn:        tlsConn,
		isConnected: true,
		client:      client,
	}
	go conn.handleReads() // запускаем чтение ошибок из соединения
	client.conn = conn
	return nil
}

// Send помещает уведомление для указанных токенов устройств в очередь на отправку и запускает
// сервис отправки, если он не был запущен.
func (client *Client) Send(ntf *Notification, tokens ...string) error {
	client.mu.RLock()
	if client.isClosed {
		client.mu.RUnlock()
		return ErrClientIsClosed
	}
	client.mu.RUnlock()
	// добавляем сообщение в очередь на отправку
	if err := client.queue.AddNotification(ntf, tokens...); err != nil {
		return err
	}
	// разбираемся с отправкой
	client.mu.RLock()
	started := client.isSendign
	client.mu.RUnlock()
	if !started {
		client.mu.Lock()
		client.isSendign = true // взводим флаг запуска сервиса
		client.mu.Unlock()
		go client.sendQueue() // запускаем отправку сообщений из очереди
	}
	return nil
}

// Close закрывает соединение с APNS-сервером. Если в качестве параметра передано true, то перед
// закрытием метод будет ждать, пока не будут отправлены все уведомления из очереди. В противном
// случае очередь будет проигнорирована и уведомления из нее могут быть не доставлены.
func (client *Client) Close(wait bool) {
	client.mu.Lock()
	client.isClosed = true // больше не принимаем новых уведомлений
	client.mu.Unlock()
	if wait {
	repeat:
		client.mu.RLock()
		started := client.isSendign
		client.mu.RUnlock()
		if started { // ждем окончания рассылки
			time.Sleep(DurationSend)
			goto repeat
		}
	}
	if client.conn != nil {
		client.conn.Close()
	}
}

// sendQueue непосредственно осуществляет отправку уведомлений на сервер, пока в очереди есть
// хотя бы одно уведомление. Если в процессе отсылки происходит ошибка соединения, то соединение
// автоматически восстанавливается.
//
// Если в очереди на отправку находится более одного уведомления, то они объединяются в один пакет
// и этот пакет отправляется либо до достижении заданной длинны, либо по окончании очереди на
// отправку.
//
// Функция отслеживает попытку запуска нескольких копий и не позволяет это делать ввиду полной
// не эффективности данного мероприятия.
func (client *Client) sendQueue() {
	// defer un(trace("[send]"))        // DEBUG
	if !client.queue.IsHasToSend() { // выходим, если нечего отправлять
		// log.Println("Nothing to send...")
		return
	}
	// отправляем сообщения на сервер
	var (
		ntf    *notification // последнее полученное на отправку уведомление
		sended uint          // количество отправленных
		buf    = getBuffer() // получаем из пулла байтовый буфер
	)
reconnect:
	for { // делаем это пока не отправим все...
		// проверяем соединение: если не установлено, то соединяемся
		if client.conn == nil || !client.conn.isConnected {
			if err := client.conn.Connect(); err != nil {
				break // выходим, если не удалось соединиться с сервером.
			}
		}
		for { // пока не отправим все
			// если уведомление уже было раньше получено, то новое не получаем
			if ntf == nil {
				ntf = client.queue.Get() // получаем уведомление из очереди
				if ntf == nil && DurationSend > 0 {
					time.Sleep(DurationSend) // если очередь пуста, то подождем немного
					ntf = client.queue.Get() // попробуем еще раз...
				}
			}
			// если больше нет уведомлений, а буфер не пустой, или после добавления
			// этого уведомления буфер переполнится, то отправляем буфер на сервер
			if (ntf == nil && buf.Len() > 0) || (buf.Len()+ntf.Len() > MaxFrameBuffer) {
				n, err := buf.WriteTo(client.conn) // отправляем буфер на сервер
				if err != nil {
					client.config.log.Println("Send error:", err)
					break // ошибка соединения - соединяемся заново
				}
				// увеличиваем время ожидания ответа после успешной отправки данных
				client.conn.SetReadDeadline(time.Now().Add(TiemoutRead))
				client.config.log.Printf("Sended %d messages (%d bytes)", sended, n)
				sended = 0 // сбрасываем счетчик отправленного
			}
			if ntf == nil { // очередь закончилась
				// log.Println("Queue is empty...")
				break reconnect // прерываем весь цикл
			}
			ntf.WriteTo(buf) // сохраняем бинарное представление уведомления в буфере
			ntf = nil        // забываем про уже отправленное
			sended++         // увеличиваем счетчик отправленного
		}
	}
	putBuffer(buf) // освобождаем буфер после работы
	client.mu.Lock()
	client.isSendign = false // сбрасываем флаг активной посылки
	client.mu.Unlock()
}
