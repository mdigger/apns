package apns

import (
	"fmt"
)

func trace(s string) string {
	fmt.Println("->", s)
	return s
}

func un(s string) {
	fmt.Println("<-", s)
}
