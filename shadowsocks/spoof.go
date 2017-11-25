package shadowsocks

type Spoof interface {
	Parse(data []byte) (dataLen uint16, err error)
	Create() (data []byte, err error)
	GetHeaderLen() (dataLen uint16, err error)
}
