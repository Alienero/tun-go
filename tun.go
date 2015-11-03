package tun

const (
	IPv6_HEADER_LENGTH = 40
)

type Tun interface {
	ReadFromChannel(ch chan []byte) error
	WriteIntoChannel(ch chan []byte) error
	Close() error
}
