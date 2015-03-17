package apns

import (
	"bytes"
	"sync"
)

// Пул байтовых буферов
var pool sync.Pool

// getBuffer возвращает пустой буфер байтов.
func getBuffer() (buf *bytes.Buffer) {
	if b := pool.Get(); b != nil {
		buf = b.(*bytes.Buffer)
		buf.Reset()
	} else {
		buf = new(bytes.Buffer)
	}
	return buf
}

// putBuffer возвращает байтовый буфер в пул буферов.
func putBuffer(buf *bytes.Buffer) {
	pool.Put(buf)
}
