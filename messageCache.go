package apns

import (
	"log"
	"sync"
	"time"
)

// messageCache - кеш для списка отправленных сообщений.
type messageCache struct {
	list []*sendMessage
	mu   sync.RWMutex
	log  *log.Logger
}

// newMessageCache возвращает новый инициализированный кеш для отправленных сообщений.
// Если при создании указать время жизни элементов в кеше, то они будут автоматически удаляться
// из него по истечении этого времени.
func newMessageCache(d time.Duration) *messageCache {
	cache := &messageCache{
		list: make([]*sendMessage, 0, 1000),
	}
	if d > 0 {
		go func() {
			for range time.NewTicker(d).C {
				if len(cache.list) == 0 { // не работаем с пустыми кешами
					return
				}
				lifeTime := time.Now().Add(-d) // время создания, после которого элементы устарели
				cache.mu.Lock()
				for i, msg := range cache.list {
					// список всегда упорядочен по дате, поэтому достаточно найти первое вхождение
					if msg.created.After(lifeTime) { // элемент добавлен после даты, с которой считается устаревшим
						if i > 0 { // если это самый первый элемент, то ничего делать не нужно
							cache.list = cache.list[i:] // удаляем все, что до этого элемента
							// if cache.log != nil {
							// 	cache.log.Printf("Cleared %d items in cache", i)
							// }
							log.Printf("Cleared %d items in cache", i)
						}
						break
					}
				}
				cache.mu.Unlock()

			}
		}()
	}
	return cache
}

// Add добавляет новое сообщение в кеш.
func (cache *messageCache) Add(messages ...*sendMessage) {
	now := time.Now() // не стоит вычислять время на каждый заход
	for _, msg := range messages {
		msg.created = now // при добавлении в кеш, сохраняем время добавления
	}
	cache.mu.Lock()
	cache.list = append(cache.list, messages...)
	cache.mu.Unlock()
}

// Len возвращает количество элементов, хранящихся в кеше.
func (cache *messageCache) Len() int {
	return len(cache.list)
}

// MoveTo переносит содержимое кеша в новый кеш.
func (cache *messageCache) MoveTo(destination *messageCache) {
	destination.Add(cache.list...)
	cache.Clear()
}

// Clear удаляет все сообщения из кеша.
func (cache *messageCache) Clear() {
	cache.mu.Lock()
	cache.list = make([]*sendMessage, 0, 1000)
	cache.mu.Unlock()
}

// FromId возвращает список тех элементов кеша, который лежать после элемента
// с указанным идентификатором. Второй параметр указывает, нужно ли исключать
// искомый элемент из этого списка или нет.
func (cache *messageCache) FromId(id uint32, exclude bool) []*sendMessage {
	for i, msg := range cache.list {
		if msg.id == id {
			if exclude {
				i++
			}
			return cache.list[i:]
		}
	}
	return nil
}
