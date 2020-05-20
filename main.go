package rtsp

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/Monibuca/engine/v2"
	. "github.com/Monibuca/engine/v2/avformat"
	"github.com/Monibuca/engine/v2/util"
	"github.com/teris-io/shortid"
)

var collection = sync.Map{}
var config = struct {
	ListenAddr string
	AutoPull   bool
	RemoteAddr string
	Timeout    int
}{":554", false, "rtsp://localhost/${streamPath}", 0}

func init() {
	InstallPlugin(&PluginConfig{
		Name:   "RTSP",
		Type:   PLUGIN_PUBLISHER | PLUGIN_HOOK,
		Config: &config,
		Run:    runPlugin,
		HotConfig: map[string]func(interface{}){
			"AutoPull": func(value interface{}) {
				config.AutoPull = value.(bool)
			},
		},
	})
}
func runPlugin() {
	OnSubscribeHooks.AddHook(func(s *Subscriber) {
		if config.AutoPull && s.Publisher == nil {
			new(RTSP).PullStream(s.StreamPath, strings.Replace(config.RemoteAddr, "${streamPath}", s.StreamPath, -1))
		}
	})
	http.HandleFunc("/rtsp/list", func(w http.ResponseWriter, r *http.Request) {
		sse := util.NewSSE(w, r.Context())
		var err error
		for tick := time.NewTicker(time.Second); err == nil; <-tick.C {
			var info []*RTSPInfo
			collection.Range(func(key, value interface{}) bool {
				rtsp := value.(*RTSP)
				pinfo := &rtsp.RTSPInfo
				info = append(info, pinfo)
				return true
			})
			err = sse.WriteJSON(info)
		}
	})
	http.HandleFunc("/rtsp/pull", func(w http.ResponseWriter, r *http.Request) {
		targetURL := r.URL.Query().Get("target")
		streamPath := r.URL.Query().Get("streamPath")
		if err := new(RTSP).PullStream(streamPath, targetURL); err == nil {
			w.Write([]byte(`{"code":0}`))
		} else {
			w.Write([]byte(fmt.Sprintf(`{"code":1,"msg":"%s"}`, err.Error())))
		}
	})
	if config.ListenAddr != "" {
		log.Fatal(ListenRtsp(config.ListenAddr))
	}
}

func ListenRtsp(addr string) error {
	defer log.Println("rtsp server start!")
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	var tempDelay time.Duration
	networkBuffer := 204800
	timeoutMillis := config.Timeout
	for {
		conn, err := listener.Accept()
		conn.(*net.TCPConn).SetNoDelay(false)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				fmt.Printf("rtsp: Accept error: %v; retrying in %v", err, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return err
		}

		tempDelay = 0
		timeoutTCPConn := &RichConn{conn, time.Duration(timeoutMillis) * time.Millisecond}
		go (&RTSP{
			ID:                 shortid.MustGenerate(),
			Conn:               timeoutTCPConn,
			connRW:             bufio.NewReadWriter(bufio.NewReaderSize(timeoutTCPConn, networkBuffer), bufio.NewWriterSize(timeoutTCPConn, networkBuffer)),
			Timeout:            config.Timeout,
			vRTPChannel:        -1,
			vRTPControlChannel: -1,
			aRTPChannel:        -1,
			aRTPControlChannel: -1,
		}).AcceptPush()
	}
	return nil
}

type RTSP struct {
	Publisher
	RTSPInfo
	RTSPClientInfo
	ID        string
	Conn      *RichConn
	connRW    *bufio.ReadWriter
	connWLock sync.RWMutex
	Type      SessionType
	TransType TransType

	SDPMap   map[string]*SDPInfo
	nonce    string
	closeOld bool
	AControl string
	VControl string
	ACodec   string
	VCodec   string
	avcsent  bool
	aacsent  bool
	Timeout  int
	// stats info
	fuBuffer []byte
	//tcp channels
	aRTPChannel         int
	aRTPControlChannel  int
	vRTPChannel         int
	vRTPControlChannel  int
	UDPServer           *UDPServer
	UDPClient           *UDPClient
	SPS                 []byte
	PPS                 []byte
	AudioSpecificConfig []byte
	Auth                func(string) string
}
type RTSPClientInfo struct {
	Agent    string
	Session  string
	authLine string
	Seq      int
}
type RTSPInfo struct {
	URL       string
	SyncCount int64
	SDPRaw    string
	InBytes   int
	OutBytes  int

	StreamInfo *StreamInfo
}

type RichConn struct {
	net.Conn
	timeout time.Duration
}

func (conn *RichConn) Read(b []byte) (n int, err error) {
	if conn.timeout > 0 {
		conn.Conn.SetReadDeadline(time.Now().Add(conn.timeout))
	} else {
		var t time.Time
		conn.Conn.SetReadDeadline(t)
	}
	return conn.Conn.Read(b)
}

func (conn *RichConn) Write(b []byte) (n int, err error) {
	if conn.timeout > 0 {
		conn.Conn.SetWriteDeadline(time.Now().Add(conn.timeout))
	} else {
		var t time.Time
		conn.Conn.SetWriteDeadline(t)
	}
	return conn.Conn.Write(b)
}
func (rtsp *RTSP) handleNALU(nalType byte, payload []byte, ts int64) {
	rtsp.SyncCount++
	vl := len(payload)
	switch nalType {
	// case NALU_SPS:
	// 	r := bytes.NewBuffer([]byte{})
	// 	r.Write(RTMP_AVC_HEAD)
	// 	util.BigEndian.PutUint16(spsHead[1:], uint16(vl))
	// 	r.Write(spsHead)
	// 	r.Write(payload)
	// case NALU_PPS:
	// 	r := bytes.NewBuffer([]byte{})
	// 	util.BigEndian.PutUint16(ppsHead[1:], uint16(vl))
	// 	r.Write(ppsHead)
	// 	r.Write(payload)
	// 	rtsp.PushVideo(0, r.Bytes())
	// avcsent = true
	case NALU_IDR_Picture:
		if !rtsp.avcsent {
			r := bytes.NewBuffer([]byte{})
			r.Write(RTMP_AVC_HEAD)
			spsHead := []byte{0xE1, 0, 0}
			util.BigEndian.PutUint16(spsHead[1:], uint16(len(rtsp.SPS)))
			r.Write(spsHead)
			r.Write(rtsp.SPS)
			ppsHead := []byte{0x01, 0, 0}
			util.BigEndian.PutUint16(ppsHead[1:], uint16(len(rtsp.PPS)))
			r.Write(ppsHead)
			r.Write(rtsp.PPS)
			rtsp.PushVideo(0, r.Bytes())
			rtsp.avcsent = true
		}
		r := bytes.NewBuffer([]byte{})
		iframeHead := []byte{0x17, 0x01, 0, 0, 0}
		util.BigEndian.PutUint24(iframeHead[2:], 0)
		r.Write(iframeHead)
		nalLength := []byte{0, 0, 0, 0}
		util.BigEndian.PutUint32(nalLength, uint32(vl))
		r.Write(nalLength)
		r.Write(payload)
		rtsp.PushVideo(uint32(ts), r.Bytes())
	case NALU_Non_IDR_Picture:
		r := bytes.NewBuffer([]byte{})
		pframeHead := []byte{0x27, 0x01, 0, 0, 0}
		util.BigEndian.PutUint24(pframeHead[2:], 0)
		r.Write(pframeHead)
		nalLength := []byte{0, 0, 0, 0}
		util.BigEndian.PutUint32(nalLength, uint32(vl))
		r.Write(nalLength)
		r.Write(payload)
		rtsp.PushVideo(uint32(ts), r.Bytes())
	}
}
func (rtsp *RTSP) handleRTP(pack *RTPPack) {
	data := pack.Buffer
	switch pack.Type {
	case RTP_TYPE_AUDIO:
		if !rtsp.aacsent {
			rtsp.PushAudio(0, append([]byte{0xAF, 0x00}, rtsp.AudioSpecificConfig...))
			rtsp.aacsent = true
		}
		cc := data[0] & 0xF
		rtphdr := 12 + cc*4
		payload := data[rtphdr:]
		auHeaderLen := (int16(payload[0]) << 8) + int16(payload[1])
		auHeaderLen = auHeaderLen >> 3
		auHeaderCount := int(auHeaderLen / 2)
		var auLenArray []int
		for iIndex := 0; iIndex < int(auHeaderCount); iIndex++ {
			auHeaderInfo := (int16(payload[2+2*iIndex]) << 8) + int16(payload[2+2*iIndex+1])
			auLen := auHeaderInfo >> 3
			auLenArray = append(auLenArray, int(auLen))
		}
		startOffset := 2 + 2*auHeaderCount
		for _, auLen := range auLenArray {
			endOffset := startOffset + auLen
			addHead := []byte{0xAF, 0x01}
			rtsp.PushAudio(0, append(addHead, payload[startOffset:endOffset]...))
			startOffset = startOffset + auLen
		}
	case RTP_TYPE_VIDEO:
		cc := data[0] & 0xF
		//rtp header
		rtphdr := 12 + cc*4

		//packet time
		ts := (int64(data[4]) << 24) + (int64(data[5]) << 16) + (int64(data[6]) << 8) + (int64(data[7]))

		//packet number
		//packno := (int64(data[6]) << 8) + int64(data[7])
		data = data[rtphdr:]
		nalType := data[0] & 0x1F

		if nalType >= 1 && nalType <= 23 {
			rtsp.handleNALU(nalType, data, ts)
		} else if nalType == 28 {
			isStart := data[1]&0x80 != 0
			isEnd := data[1]&0x40 != 0
			nalType := data[1] & 0x1F
			//nri := (data[1]&0x60)>>5
			nal := data[0]&0xE0 | data[1]&0x1F
			if isStart {
				rtsp.fuBuffer = []byte{0}
			}
			rtsp.fuBuffer = append(rtsp.fuBuffer, data[2:]...)
			if isEnd {
				rtsp.fuBuffer[0] = nal
				rtsp.handleNALU(nalType, rtsp.fuBuffer, ts)
			}
		}
	}
}
