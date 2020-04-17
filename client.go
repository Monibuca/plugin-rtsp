package rtspplugin

import (
	"crypto/md5"
	b64 "encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	VideoWidth  int
	VideoHeight int
	spropReg    *regexp.Regexp
	configReg   *regexp.Regexp
)

func init() {
	spropReg, _ = regexp.Compile("sprop-parameter-sets=([^;]+)")
	configReg, _ = regexp.Compile("config=([^;]+)")
}

type RtspClient struct {
	socket              net.Conn
	OutGoing            chan []byte //out chanel
	Signals             chan bool   //Signals quit
	host                string      //host
	port                string      //port
	uri                 string      //url
	auth                bool        //aut
	login               string
	password            string   //password
	session             string   //rtsp session
	responce            string   //responce string
	bauth               string   //string b auth
	track               []string //rtsp track
	cseq                int      //qury number
	videow              int
	videoh              int
	SPS                 []byte
	PPS                 []byte
	Header              string
	AudioSpecificConfig []byte
}

//RtspClientNew 返回空的初始化对象
func RtspClientNew(bufferLength int) *RtspClient {
	Obj := &RtspClient{
		cseq:     1,                               //查询起始号码
		Signals:  make(chan bool, 1),              //一个消息缓冲通道
		OutGoing: make(chan []byte, bufferLength), //输出通道
	}
	return Obj
}

func (this *RtspClient) Client(rtsp_url string) (bool, string) {
	//Check back url
	if !this.ParseUrl(rtsp_url) {
		return false, "Incorrect url"
	}
	//Install connect to camera
	if !this.Connect() {
		return false, "cannot connect"
	}
	//Phase 1 options camera phase 1
	//Send options request
	if !this.Write("OPTIONS " + this.uri + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\n\r\n") {
		return false, "Unable to send options message"
	}
	//Read the response to the options request
	if status, message := this.Read(); !status {
		return false, "Unable to read options response connection lost"
	} else if status && strings.Contains(message, "Digest") {
		if !this.AuthDigest("OPTIONS", message) {
			return false, "Summary of authorization required"
		}
	} else if status && strings.Contains(message, "Basic") {
		if !this.AuthBasic("OPTIONS", message) {
			return false, "Need certification Basic"
		}
	} else if !strings.Contains(message, "200") {
		return false, "error OPTIONS not status code 200 OK " + message
	}

	////////////PHASE 2 DESCRIBE
	log.Println("DESCRIBE " + this.uri + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + this.bauth + "\r\n\r\n")
	if !this.Write("DESCRIBE " + this.uri + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + this.bauth + "\r\n\r\n") {
		return false, "Unable to send query DESCRIBE"
	}
	if status, message := this.Read(); !status {
		return false, "Can't read response for decscribe connection loss?"
	} else if status && strings.Contains(message, "Digest") {
		if !this.AuthDigest("DESCRIBE", message) {
			return false, "Summary of authorization required"
		}
	} else if status && strings.Contains(message, "Basic") {
		if !this.AuthBasic("DESCRIBE", message) {
			return false, "Basis of authorization required"
		}
	} else if !strings.Contains(message, "200") {
		return false, "error DESCRIBE not status code 200 OK " + message
	} else {
		this.Header = message
		this.track = this.ParseMedia(message)

	}
	if len(this.track) == 0 {
		return false, "error track not found "
	}
	//PHASE 3 SETUP
	log.Println("SETUP " + this.uri + "/" + this.track[0] + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\nTransport: RTP/AVP/TCP;unicast;interleaved=0-1" + this.bauth + "\r\n\r\n")
	if !this.Write("SETUP " + this.uri + "/" + this.track[0] + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\nTransport: RTP/AVP/TCP;unicast;interleaved=0-1" + this.bauth + "\r\n\r\n") {
		return false, ""
	}
	if status, message := this.Read(); !status {
		return false, "Unable to read response for missing setup connection."

	} else if !strings.Contains(message, "200") {
		if strings.Contains(message, "401") {
			str := this.AuthDigest_Only("SETUP", message)
			if !this.Write("SETUP " + this.uri + "/" + this.track[0] + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\nTransport: RTP/AVP/TCP;unicast;interleaved=0-1" + this.bauth + str + "\r\n\r\n") {
				return false, ""
			}
			if status, message := this.Read(); !status {
				return false, "Unable to read response for missing setup connection."

			} else if !strings.Contains(message, "200") {

				return false, "error SETUP not status code 200 OK " + message

			} else {
				this.session = ParseSession(message)
			}
		} else {
			return false, "error SETUP not status code 200 OK " + message
		}
	} else {
		log.Println(message)
		this.session = ParseSession(message)
		log.Println(this.session)
	}
	if len(this.track) > 1 {

		if !this.Write("SETUP " + this.uri + "/" + this.track[1] + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\nTransport: RTP/AVP/TCP;unicast;interleaved=2-3" + "\r\nSession: " + this.session + this.bauth + "\r\n\r\n") {
			return false, ""
		}
		if status, message := this.Read(); !status {
			return false, "Unable to read response for missing setup audio connection."

		} else if !strings.Contains(message, "200") {
			if strings.Contains(message, "401") {
				str := this.AuthDigest_Only("SETUP", message)
				if !this.Write("SETUP " + this.uri + "/" + this.track[1] + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\nTransport: RTP/AVP/TCP;unicast;interleaved=2-3" + this.bauth + str + "\r\n\r\n") {
					return false, ""
				}
				if status, message := this.Read(); !status {
					return false, "Unable to read response for missing setup audio connection."

				} else if !strings.Contains(message, "200") {

					return false, "error SETUP not status code 200 OK " + message

				} else {
					log.Println(message)
					this.session = ParseSession(message)
				}
			} else {
				return false, "error SETUP not status code 200 OK " + message
			}
		} else {
			log.Println(message)
			this.session = ParseSession(message)
		}
	}

	//PHASE 4 SETUP
	log.Println("PLAY " + this.uri + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\nSession: " + this.session + this.bauth + "\r\n\r\n")
	if !this.Write("PLAY " + this.uri + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\nSession: " + this.session + this.bauth + "\r\n\r\n") {
		return false, ""
	}
	if status, message := this.Read(); !status {
		return false, "Unable to read play response lost connection"

	} else if !strings.Contains(message, "200") {
		//return false, "Ошибка PLAY not status code 200 OK " + message
		if strings.Contains(message, "401") {
			str := this.AuthDigest_Only("PLAY", message)
			if !this.Write("PLAY " + this.uri + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\nSession: " + this.session + this.bauth + str + "\r\n\r\n") {
				return false, ""
			}
			if status, message := this.Read(); !status {
				return false, "Unable to read play response lost connection"

			} else if !strings.Contains(message, "200") {

				return false, "error PLAY not status code 200 OK " + message

			} else {
				//this.session = ParseSession(message)
				log.Print(message)
				go this.RtspRtpLoop()
				return true, "ok"
			}
		} else {
			return false, "error PLAY not status code 200 OK " + message
		}
	} else {
		log.Print(message)
		go this.RtspRtpLoop()
		return true, "ok"
	}
	return false, "other error"
}

/*
	The RTP header has the following format:
    0                   1                   2                   3
    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |V=2|P|X|  CC   |M|     PT      |       sequence number         |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                           timestamp                           |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |           synchronization source (SSRC) identifier            |
   +=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+=+
   |            contributing source (CSRC) identifiers             |
   |                             ....                              |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   version (V): 2 bits
      This field identifies the version of RTP.  The version defined by
      this specification is two (2).  (The value 1 is used by the first
      draft version of RTP and the value 0 is used by the protocol
      initially implemented in the "vat" audio tool.)
   padding (P): 1 bit
      If the padding bit is set, the packet contains one or more
      additional padding octets at the end which are not part of the
      payload.  The last octet of the padding contains a count of how
      many padding octets should be ignored, including itself.  Padding
      may be needed by some encryption algorithms with fixed block sizes
      or for carrying several RTP packets in a lower-layer protocol data
      unit.
   extension (X): 1 bit
      If the extension bit is set, the fixed header MUST be followed by
      exactly one header extension, with a format defined in Section
      5.3.1.
*/
func (this *RtspClient) RtspRtpLoop() {
	defer func() {
		this.Signals <- true
	}()
	header := make([]byte, 4)
	payload := make([]byte, 4096)
	//sync := make([]byte, 256)
	sync_b := make([]byte, 1)
	timer := time.Now()
	for {
		if int(time.Now().Sub(timer).Seconds()) > 50 {
			if !this.Write("OPTIONS " + this.uri + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + "\r\nSession: " + this.session + this.bauth + "\r\n\r\n") {
				return
			}
			timer = time.Now()
		}
		this.socket.SetDeadline(time.Now().Add(50 * time.Second))
		//read rtp hdr 4
		if n, err := io.ReadFull(this.socket, header); err != nil || n != 4 {
			//rtp hdr read error
			return
		}
		//log.Println(header)
		if header[0] != 36 {
			//log.Println("desync?", this.host)
			for {
				///////////////////////////skeep/////////////////////////////////////
				if n, err := io.ReadFull(this.socket, sync_b); err != nil && n != 1 {
					return
				} else if sync_b[0] == 36 {
					header[0] = 36
					if n, err := io.ReadFull(this.socket, header[1:]); err != nil && n == 3 {
						return
					}
					break
				}
			}
			/*
				//вычитываем 256 в попытке отсять мусор обрезать RTSP
				if string(header) == "RTSP" {
					if n, err := io.ReadFull(this.socket, sync); err != nil && n == 256 {
						return
					} else {
						rtsp_rtp := []byte(strings.Split(string(sync), "\r\n\r\n")[1])
						//отправим все что есть в буфере
						this.SendBufer(rtsp_rtp)
						continue
					}
				} else {
					log.Println("full desync")
					return
				}
			*/
		}

		payloadLen := (int)(header[2])<<8 + (int)(header[3])
		//log.Println("payloadLen", payloadLen)
		if payloadLen > 4096 || payloadLen < 12 {
			log.Println("desync", this.uri, payloadLen)
			return
		}
		if n, err := io.ReadFull(this.socket, payload[:payloadLen]); err != nil || n != payloadLen {
			return
		} else {
			this.OutGoing <- append(header, payload[:n]...)
		}
	}

}

//unsafe!
func (this *RtspClient) SendBufer(bufer []byte) {
	//Here you need to send all the packages from the send all buffer?
	payload := make([]byte, 4096)
	for {
		if len(bufer) < 4 {
			log.Fatal("bufer small")
		}
		dataLength := (int)(bufer[2])<<8 + (int)(bufer[3])
		if dataLength > len(bufer)+4 {
			if n, err := io.ReadFull(this.socket, payload[:dataLength-len(bufer)+4]); err != nil {
				return
			} else {
				this.OutGoing <- append(bufer, payload[:n]...)
				return
			}

		} else {
			this.OutGoing <- bufer[:dataLength+4]
			bufer = bufer[dataLength+4:]
		}
	}
}
func (this *RtspClient) Connect() bool {
	d := &net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.Dial("tcp", this.host+":"+this.port)
	if err != nil {
		return false
	}
	this.socket = conn
	return true
}
func (this *RtspClient) Write(message string) bool {
	this.cseq += 1
	if _, e := this.socket.Write([]byte(message)); e != nil {
		return false
	}
	return true
}
func (this *RtspClient) Read() (bool, string) {
	buffer := make([]byte, 4096)
	if nb, err := this.socket.Read(buffer); err != nil || nb <= 0 {
		log.Println("socket read failed", err)
		return false, ""
	} else {
		return true, string(buffer[:nb])
	}
}
func (this *RtspClient) AuthBasic(phase string, message string) bool {
	this.bauth = "\r\nAuthorization: Basic " + b64.StdEncoding.EncodeToString([]byte(this.login+":"+this.password))
	if !this.Write(phase + " " + this.uri + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + this.bauth + "\r\n\r\n") {
		return false
	}
	if status, message := this.Read(); status && strings.Contains(message, "200") {
		this.track = this.ParseMedia(message)
		return true
	}
	return false
}
func (this *RtspClient) AuthDigest(phase string, message string) bool {
	nonce := ParseDirective(message, "nonce")
	realm := ParseDirective(message, "realm")
	hs1 := GetMD5Hash(this.login + ":" + realm + ":" + this.password)
	hs2 := GetMD5Hash(phase + ":" + this.uri)
	responce := GetMD5Hash(hs1 + ":" + nonce + ":" + hs2)
	dauth := "\r\n" + `Authorization: Digest username="` + this.login + `", realm="` + realm + `", nonce="` + nonce + `", uri="` + this.uri + `", response="` + responce + `"`
	if !this.Write(phase + " " + this.uri + " RTSP/1.0\r\nCSeq: " + strconv.Itoa(this.cseq) + dauth + "\r\n\r\n") {
		return false
	}
	if status, message := this.Read(); status && strings.Contains(message, "200") {
		this.track = this.ParseMedia(message)
		return true
	}
	return false
}
func (this *RtspClient) AuthDigest_Only(phase string, message string) string {
	nonce := ParseDirective(message, "nonce")
	realm := ParseDirective(message, "realm")
	hs1 := GetMD5Hash(this.login + ":" + realm + ":" + this.password)
	hs2 := GetMD5Hash(phase + ":" + this.uri)
	responce := GetMD5Hash(hs1 + ":" + nonce + ":" + hs2)
	dauth := "\r\n" + `Authorization: Digest username="` + this.login + `", realm="` + realm + `", nonce="` + nonce + `", uri="` + this.uri + `", response="` + responce + `"`
	return dauth
}
func (this *RtspClient) ParseUrl(rtsp_url string) bool {

	u, err := url.Parse(rtsp_url)
	if err != nil {
		return false
	}
	phost := strings.Split(u.Host, ":")
	this.host = phost[0]
	if len(phost) == 2 {
		this.port = phost[1]
	} else {
		this.port = "554"
	}
	this.login = u.User.Username()
	this.password, this.auth = u.User.Password()
	if u.RawQuery != "" {
		this.uri = "rtsp://" + this.host + ":" + this.port + u.Path + "?" + string(u.RawQuery)
	} else {
		this.uri = "rtsp://" + this.host + ":" + this.port + u.Path
	}
	return true
}
func (this *RtspClient) Close() {
	if this.socket != nil {
		this.socket.Close()
	}
}
func ParseDirective(header, name string) string {
	index := strings.Index(header, name)
	if index == -1 {
		return ""
	}
	start := 1 + index + strings.Index(header[index:], `"`)
	end := start + strings.Index(header[start:], `"`)
	return strings.TrimSpace(header[start:end])
}
func ParseSession(header string) string {
	mparsed := strings.Split(header, "\r\n")
	for _, element := range mparsed {
		if strings.Contains(element, "Session:") {
			if strings.Contains(element, ";") {
				fist := strings.Split(element, ";")[0]
				return fist[9:]
			} else {
				return element[9:]
			}
		}
	}
	return ""
}

// func ParseMedia(header string) []string {
// 	letters := []string{}
// 	mparsed := strings.Split(header, "\r\n")
// 	paste := ""

// 	// if true {
// 	// 	log.Println("headers", header)
// 	// }

// 	for _, element := range mparsed {
// 		if strings.Contains(element, "a=control:") && !strings.Contains(element, "*") {
// 			paste = element[10:]
// 			if strings.Contains(element, "/") {
// 				striped := strings.Split(element, "/")
// 				paste = striped[len(striped)-1]
// 			}
// 			letters = append(letters, paste)
// 		}

// 		dimensionsPrefix := "a=x-dimensions:"
// 		if strings.HasPrefix(element, dimensionsPrefix) {
// 			dims := []int{}
// 			for _, s := range strings.Split(element[len(dimensionsPrefix):], ",") {
// 				v := 0
// 				fmt.Sscanf(s, "%d", &v)
// 				if v <= 0 {
// 					break
// 				}
// 				dims = append(dims, v)
// 			}
// 			if len(dims) == 2 {
// 				VideoWidth = dims[0]
// 				VideoHeight = dims[1]
// 			}
// 		}
// 		if strings.Contains(element, "sprop-parameter-sets") {
// 			group := spropReg.FindAllStringSubmatch(element, -1)
// 			log.Println(group[1])
// 		}
// 	}
// 	return letters
// }
func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}
func (this *RtspClient) ParseMedia(header string) []string {
	letters := []string{}
	log.Println(header)
	mparsed := strings.Split(header, "\r\n")
	paste := ""
	for _, element := range mparsed {
		if strings.Contains(element, "a=control:") && !strings.Contains(element, "*") {
			paste = element[10:]
			if strings.Contains(element, "/") {
				striped := strings.Split(element, "/")
				paste = striped[len(striped)-1]
			}
			letters = append(letters, paste)
		}

		dimensionsPrefix := "a=x-dimensions:"
		if strings.HasPrefix(element, dimensionsPrefix) {
			dims := []int{}
			for _, s := range strings.Split(element[len(dimensionsPrefix):], ",") {
				v := 0
				fmt.Sscanf(s, "%d", &v)
				if v <= 0 {
					break
				}
				dims = append(dims, v)
			}
			if len(dims) == 2 {
				this.videow = dims[0]
				this.videoh = dims[1]
			}
		}
		group := spropReg.FindAllStringSubmatch(element, -1)
		if len(group) > 0 {
			group := strings.Split(group[0][1], ",")
			this.SPS, _ = b64.StdEncoding.DecodeString(group[0])
			this.PPS, _ = b64.StdEncoding.DecodeString(group[1])
		} else if group = configReg.FindAllStringSubmatch(element, -1); len(group) > 0 {
			this.AudioSpecificConfig, _ = hex.DecodeString(group[0][1])
		}
	}
	return letters
}
