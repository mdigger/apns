package apns

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"
	"time"
)

const MaxFrameBuffer = 65535                // максимальный размер буфера (TCP-фрейма) в байтах
var FramingTimeout = 100 * time.Millisecond // время задержки отправки сообщений

// frameBuffer буфер для отправки нескольких сообщений одним пакетом.
// При добавлении в буфер новых данные, если они не умещаются в заданный размер,
// вызывается функция их автоматической отправки, указанная в качестве send.
type frameBuffer struct {
	buf     bytes.Buffer  // непосредственно буфер отправляемых данных
	mu      sync.RWMutex  // блокировщик на чтение и запись в буфер
	counter uint32        // счетчик отправленных сообщений
	timer   *time.Timer   // таймер задержки отправки содержимого буфера
	Size    int           // максимально допустимый размер буфера
	Delay   time.Duration // время задержки отправки сообщений
	Cache   *messageCache // кеш находящихся в данный момент в буфере элементов
	Send    func()        // функция для отправки содержимого буфера
}

// Add добавляет содержимое сообщения в кеш для отправки.
//
// Перед добавление данных проверяется, что максимальный размер буфера после этого не превысит
// заданный размер. Если это не так, то предварительно накопленные в буфере данные автоматически
// отсылаются перед добавлением.
func (fb *frameBuffer) Add(msgs ...*sendMessage) error {
	fb.mu.Lock()

	if fb.timer != nil {
		fb.timer.Stop() // останавливаем таймер, чтобы, не дай бог, не сработал параллельно
	}
	for _, msg := range msgs {
		if msg.id == 0 {
			fb.counter++
			msg.id = fb.counter // присваиваем уникальный идентификатор
		}
		data := msg.Byte()                      // получаем сообщение в байтовом представлении.
		if len(data)+fb.buf.Len()+5 > fb.Size { // проверяем, что буфер может вместить новые данные без переполнения
			fb.mu.Unlock()
			fb.Send() // не влезает - нужно сначала отправить уже накопленные данные
			fb.mu.Lock()
		}
		if err := binary.Write(&fb.buf, binary.BigEndian, uint8(2)); err != nil { // записываем тип блока
			fb.mu.Unlock()
			return err
		}
		if err := binary.Write(&fb.buf, binary.BigEndian, uint32(len(data))); err != nil { // записываем длинну данных
			fb.mu.Unlock()
			return err
		}
		if _, err := fb.buf.Write(data); err != nil { // записываем сами данные
			fb.mu.Unlock()
			return err
		}
		if fb.Cache == nil {
			fb.Cache = newMessageCache(0) // инициализируем локальный кеш, если его не было
		}
		fb.Cache.Add(msg) // добавляем сообщение в локальный кеш
	}
	// разбираемся с отправкой
	if fb.Delay == 0 { // если задержка не установлена, то отправляем сразу
		fb.mu.Unlock()
		fb.Send()
		return nil
	}
	// устанавливаем или сдвигаем, если уже был установлен, таймер с задержкой отправки
	if fb.timer == nil {
		fb.timer = time.AfterFunc(fb.Delay, fb.Send)
	} else {
		fb.timer.Reset(fb.Delay)
	}
	fb.mu.Unlock()
	return nil
}

// WriteTo записывает содержимое буфера в поток и сбрасывается.
func (fb *frameBuffer) WriteTo(w io.Writer) (int64, error) {
	fb.mu.Lock()
	n, err := fb.buf.WriteTo(w)
	fb.mu.Unlock()
	return n, err
}
