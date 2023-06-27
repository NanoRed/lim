package protocol

type Action uint8

const (
	ActResponse Action = iota
	ActHandshake
	ActLabel
	ActMulticast
)
