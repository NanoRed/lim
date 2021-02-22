package connection

import (
	"net"
	"time"
)

// Conn connection interface
type Conn interface {
	net.Conn
	Label(string)
	Dislabel(string)
	ListLabel() []string
	Health() bool
	SafetyWrite(timeout time.Duration, b []byte) (n int, err error)
}
