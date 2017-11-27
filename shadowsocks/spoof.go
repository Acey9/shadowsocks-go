package shadowsocks

type Spoof interface {
	SuffixLen(data []byte) (suffixLen uint16, err error)
	PrefixLen() (prefixLen uint16, err error)
	SpoofData() (data []byte, err error)
}
