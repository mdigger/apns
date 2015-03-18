package apns

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrBadResponseSize = errors.New("bad apple error size")

// Ошибка, возвращаемая сервером APNS.
type apnsError struct {
	Type   uint8
	Status uint8
	Id     uint32
}

func (e apnsError) Error() string {
	if e.Id != 0 {
		return fmt.Sprintf("APNS %s [message id %d]", apnsErrorMessages[e.Status], e.Id)
	}
	return fmt.Sprintf("APNS %s", apnsErrorMessages[e.Status])
}

func parseAPNSError(data []byte) error {
	if len(data) != 6 {
		return ErrBadResponseSize
	}
	var err apnsError
	binary.Read(bytes.NewReader(data), binary.BigEndian, &err)
	return err
}

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
