package shadowsocks

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"time"
)

const (
	HttPRequest       = "POST / HTTP/1.1\r\nCookie: SIN="
	HttpHeaderLenSize = 2
)

var HttpUserAgent = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/62.0.3202.89 Safari/537.36",
	"Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1; en) Opera 9.50",
	"Mozilla/5.0 (Windows NT 6.1; WOW64; rv:34.0) Gecko/20100101 Firefox/34.0",
	"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/534.57.2 (KHTML, like Gecko) Version/5.1.7 Safari/534.57.2",
	"Mozilla/5.0 (Windows NT 6.1; WOW64; Trident/7.0; rv:11.0) like Gecko",
	"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/536.11 (KHTML, like Gecko) Chrome/20.0.1132.11 TaoBrowser/2.0 Safari/536.11",
	"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.1 (KHTML, like Gecko) Chrome/21.0.1180.71 Safari/537.1 LBBROWSER",
	"Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/38.0.2125.122 UBrowser/4.0.3214.0 Safari/537.36",
}

var HttpHosts = []string{
	"www.people.com.cn",
	"www.xinhuanet.com",
	"www.cctv.com",
	"www.cri.cn",
	"cn.chinadaily.com.cn",
	"www.ce.cn",
	"www.gmw.cn",
	"www.cnr.cn",
	"www.youth.cn",
	"www.baidu.com",
	"www.163.com",
	"www.sina.com.cn",
	"www.17173.com",
	"www.iqiyi.com",
	"www.sohu.com",
	"blog.163.com",
	"weibo.com",
	"www.newsmth.net",
	"www.zhihu.com",
	"www.ifeng.com",
	"www.xiachufang.com",
	"www.douban.com",
}

var HttpHeader = "\r\nAccept: */*\r\nAccept-Language: en-US\r\nConnection: Keep-Alive\r\nContent-Type: application/text\r\n\r\n"

type Http struct {
}

func (http *Http) SuffixLen(data []byte) (suffixLen uint16, err error) {
	prefixLen, _ := http.PrefixLen()
	suffixLen = binary.LittleEndian.Uint16(data[prefixLen-HttpHeaderLenSize : prefixLen])
	return
}

func (http *Http) PrefixLen() (prefixLen uint16, err error) {
	prefixLen = uint16(len(HttPRequest)) + HttpHeaderLenSize
	return
}

func (http *Http) SpoofData() (data []byte, err error) {
	ua := http.getUa()
	host := http.getHost()
	cookie := http.getCookie()

	hbuf := bytes.Buffer{}
	hbuf.WriteString(cookie)
	hbuf.WriteString("\r\nhost: ")
	hbuf.WriteString(host)
	hbuf.WriteString("\r\nUser-Agent: ")
	hbuf.WriteString(ua)
	hbuf.WriteString(HttpHeader)

	lenBuf := make([]byte, HttpHeaderLenSize)
	binary.LittleEndian.PutUint16(lenBuf, uint16(len(hbuf.String())))

	dbuf := bytes.Buffer{}
	dbuf.WriteString(HttPRequest)
	dbuf.Write(lenBuf)
	dbuf.WriteString(hbuf.String())
	data = dbuf.Bytes()
	return
}

func (http *Http) getCookie() string {
	ts := time.Now().UnixNano()
	return fmt.Sprintf(".%d;", ts)
}

func (http *Http) getUa() string {
	rand.Seed(time.Now().UnixNano())
	return HttpUserAgent[rand.Intn(len(HttpUserAgent))]
}

func (http *Http) getHost() string {
	rand.Seed(time.Now().UnixNano())
	return HttpHosts[rand.Intn(len(HttpHosts))]
}
