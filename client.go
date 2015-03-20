package apns

import (
	"errors"
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
	// Время задержки отправки сообщений по умолчанию.
	DurationSend = 100 * time.Millisecond
	// Ошибка добавления уведомления на отправку для закрытого клиента.
	ErrClientIsClosed = errors.New("client is closed")
)

type Client struct {
	conn      *Conn              // соединение с сервером
	config    *Config            // конфигурация и сертификаты
	host      string             // адрес сервера
	queue     *notificationQueue // список уведомлений для отправки
	isSendign bool               // флаг активности отправки
	isClosed  bool               // флаг закрытия клиента
	mu        sync.RWMutex       // блокировка доступа к флагу посылки
	Delay     time.Duration      // время задержки отправки сообщений
}

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
		Delay:  DurationSend,
	}
	client.conn = NewConn(client)
	return client
}

// Send отправляет сообщение на указанные токены устройств.
func (client *Client) Send(ntf *Notification, tokens ...[]byte) error {
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
			time.Sleep(client.Delay)
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
// и этот пакет отправляется либо до достижении заданной длинны, либо по окончании очереди на отправку.
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
				panic("unknown network error")
			}
		}
		for { // пока не отправим все
			// если уведомление уже было раньше получено, то новое не получаем
			if ntf == nil {
				ntf = client.queue.Get() // получаем уведомление из очереди
				if ntf == nil && client.Delay > 0 {
					time.Sleep(client.Delay) // если очередь пуста, то подождем немного
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
			ntf.WriteTo(buf)        // сохраняем бинарное представление уведомления в буфере
			ntf.Sended = time.Now() // помечаем время отправки
			ntf = nil               // забываем про уже отправленное
			sended++                // увеличиваем счетчик отправленного
		}
	}
	putBuffer(buf) // освобождаем буфер после работы
	client.mu.Lock()
	client.isSendign = false // сбрасываем флаг активной посылки
	client.mu.Unlock()
}
