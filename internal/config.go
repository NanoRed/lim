package internal

import "time"

var (
	ConnReadDuration  time.Duration = time.Second * 10
	ConnWriteTimeout  time.Duration = time.Second * 3
	ResponseTimeout   time.Duration = time.Second * 3
	HeartbeatInterval time.Duration = time.Second * 3
)
