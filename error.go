package apns

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// errBadResponseSize ошибка
var errBadResponseSize = errors.New("bad apple error size")

// Ошибка, возвращаемая сервером APNS.
type apnsError struct {
	Type   uint8
	Status uint8
	ID     uint32
}

// Error возвращает строковое представление ошибки.
func (e apnsError) Error() string {
	if e.ID != 0 {
		return fmt.Sprintf("APNS %s [message id %d]", apnsErrorMessages[e.Status], e.ID)
	}
	return fmt.Sprintf("APNS %s", apnsErrorMessages[e.Status])
}

// parseAPNSError позволяет создать описание ошибки из набора байт, полученного от сервера Apple.
func parseAPNSError(data []byte) error {
	if len(data) != 6 {
		return errBadResponseSize
	}
	var err apnsError
	binary.Read(bytes.NewReader(data), binary.BigEndian, &err)
	return err
}

// apnsErrorMessages описывает известные мне на данный момент времени коды ошибок и их текстовое
// представление.
var apnsErrorMessages = map[uint8]string{
	0:   "No Errors",
	1:   "Processing Error",
	2:   "Missing Device Token",
	3:   "Missing Topic",
	4:   "Missing Payload",
	5:   "Invalid Token Size",
	6:   "Invalid Topic Size",
	7:   "Invalid Payload Size",
	8:   "Invalid Token",
	10:  "Shutdown",
	128: "Invalid Frame Item Id", // не документировано, но найдено в ходе тестов
	255: "Unknown error",
}
