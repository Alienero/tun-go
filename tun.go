package tun

const (
	IPv6_HEADER_LENGTH = 40
)

type Tun interface {
	Read(ch chan []byte) error
	Write(ch chan []byte) error
	Close() error
}
