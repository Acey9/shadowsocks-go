package shadowsocks

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
)

const (
	OneTimeAuthMask byte = 0x10
	AddrMask        byte = 0xf
)

const (
	STAGE_INIT       = 0
	STAGE_ADDR       = 1
	STAGE_UDP_ASSOC  = 2
	STAGE_DNS        = 3
	STAGE_CONNECTING = 4
	STAGE_STREAM     = 5
)

type Conn struct {
	net.Conn
	*Cipher
	readBuf    []byte
	writeBuf   []byte
	chunkId    uint32
	Stage      uint8
	SpoofProto Spoof
}

func NewConn(c net.Conn, cipher *Cipher) *Conn {
	spoof := &Http{}
	return &Conn{
		Conn:       c,
		Cipher:     cipher,
		SpoofProto: spoof,
		readBuf:    leakyBuf.Get(),
		writeBuf:   leakyBuf.Get(),
		Stage:      STAGE_INIT,
	}
}

func (c *Conn) SetStage(stage uint8) {
	c.Stage = stage
}

func (c *Conn) Close() error {
	leakyBuf.Put(c.readBuf)
	leakyBuf.Put(c.writeBuf)
	return c.Conn.Close()
}

func RawAddr(addr string) (buf []byte, err error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("shadowsocks: address error %s %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("shadowsocks: invalid port %s", addr)
	}

	hostLen := len(host)
	l := 1 + 1 + hostLen + 2 // addrType + lenByte + address + port
	buf = make([]byte, l)
	buf[0] = 3             // 3 means the address is domain name
	buf[1] = byte(hostLen) // host address length  followed by host address
	copy(buf[2:], host)
	binary.BigEndian.PutUint16(buf[2+hostLen:2+hostLen+2], uint16(port))
	return
}

// This is intended for use by users implementing a local socks proxy.
// rawaddr shoud contain part of the data in socks request, starting from the
// ATYP field. (Refer to rfc1928 for more information.)
func DialWithRawAddr(rawaddr []byte, server string, cipher *Cipher) (c *Conn, err error) {
	conn, err := net.Dial("tcp", server)
	if err != nil {
		return
	}
	c = NewConn(conn, cipher)
	if cipher.ota {
		if c.enc == nil {
			if _, err = c.initEncrypt(); err != nil {
				return
			}
		}
		// since we have initEncrypt, we must send iv manually
		conn.Write(cipher.iv)
		rawaddr[0] |= OneTimeAuthMask
		rawaddr = otaConnectAuth(cipher.iv, cipher.key, rawaddr)
	}
	c.SetStage(STAGE_ADDR)
	if _, err = c.write(rawaddr); err != nil {
		c.Close()
		return nil, err
	}
	c.SetStage(STAGE_CONNECTING)
	return
}

// addr should be in the form of host:port
func Dial(addr, server string, cipher *Cipher) (c *Conn, err error) {
	ra, err := RawAddr(addr)
	if err != nil {
		return
	}
	return DialWithRawAddr(ra, server, cipher)
}

func (c *Conn) GetIv() (iv []byte) {
	iv = make([]byte, len(c.iv))
	copy(iv, c.iv)
	return
}

func (c *Conn) GetKey() (key []byte) {
	key = make([]byte, len(c.key))
	copy(key, c.key)
	return
}

func (c *Conn) IsOta() bool {
	return c.ota
}

func (c *Conn) GetAndIncrChunkId() (chunkId uint32) {
	chunkId = c.chunkId
	c.chunkId += 1
	return
}

func (c *Conn) Read(b []byte) (n int, err error) {

	if c.Stage == STAGE_ADDR && !c.ota {
		_, err = c.readSpoofHeader()
		if err != nil {
			return
		}
		c.SetStage(STAGE_CONNECTING)
	}

	if c.dec == nil {
		iv := make([]byte, c.info.ivLen)
		if _, err = io.ReadFull(c.Conn, iv); err != nil {
			return
		}
		if err = c.initDecrypt(iv); err != nil {
			return
		}
		if len(c.iv) == 0 {
			c.iv = iv
		}
	}

	cipherData := c.readBuf
	if len(b) > len(cipherData) {
		cipherData = make([]byte, len(b))
	} else {
		cipherData = cipherData[:len(b)]
	}

	n, err = c.Conn.Read(cipherData)
	if n > 0 {
		c.decrypt(b[0:n], cipherData[0:n])
	}
	return
}

func (c *Conn) Write(b []byte) (n int, err error) {
	nn := len(b)
	if c.ota {
		chunkId := c.GetAndIncrChunkId()
		b = otaReqChunkAuth(c.iv, chunkId, b)
	}
	headerLen := len(b) - nn

	n, err = c.write(b)
	// Make sure <= 0 <= len(b), where b is the slice passed in.
	if n >= headerLen {
		n -= headerLen
	}
	return
}

func (c *Conn) write(b []byte) (n int, err error) {
	var iv []byte
	if c.enc == nil {
		iv, err = c.initEncrypt()
		if err != nil {
			return
		}
	}

	cipherData := c.writeBuf
	dataSize := len(b) + len(iv)
	if dataSize > len(cipherData) {
		cipherData = make([]byte, dataSize)
	} else {
		cipherData = cipherData[:dataSize]
	}

	if iv != nil {
		// Put initialization vector in buffer, do a single write to send both
		// iv and data.
		copy(cipherData, iv)
	}

	c.encrypt(cipherData[len(iv):], b)
	if c.Stage == STAGE_ADDR && !c.ota {
		_, err = c.writeSpoofHeader()
		if err != nil {
			return
		}
	}
	n, err = c.Conn.Write(cipherData)
	return
}

func (c *Conn) writeSpoofHeader() (n int, err error) {
	if !isSpoofProtocol {
		return
	}
	spoofData, err := c.SpoofProto.SpoofData()
	if err == nil {
		n, err = c.Conn.Write(spoofData)
	}
	return
}

func (c *Conn) readSpoofHeader() (n int, err error) {
	if !isSpoofProtocol {
		return
	}
	prefixLen, _ := c.SpoofProto.PrefixLen()
	buf := make([]byte, prefixLen)
	n, err = c.Conn.Read(buf)
	if err != nil || n != int(prefixLen) {
		return
	}
	suffixLen, _ := c.SpoofProto.SuffixLen(buf)
	buf = make([]byte, suffixLen)
	n, err = c.Conn.Read(buf)
	if err != nil || n != int(suffixLen) {
		return
	}
	return
}
