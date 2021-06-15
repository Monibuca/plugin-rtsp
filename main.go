package rtsp

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"github.com/teris-io/shortid"
)

var collection sync.Map
var config = struct {
	ListenAddr   string
	AutoPull     bool
	RemoteAddr   string
	Timeout      int
	Reconnect    bool
	AutoPullList []*struct {
		URL        string
		StreamPath string
	}
}{":554", false, "rtsp://localhost/${streamPath}", 0, false, nil}

func init() {
	InstallPlugin(&PluginConfig{
		Name:   "RTSP",
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

	http.HandleFunc("/api/rtsp/list", func(w http.ResponseWriter, r *http.Request) {
		sse := NewSSE(w, r.Context())
		var err error
		for tick := time.NewTicker(time.Second); err == nil; <-tick.C {
			var info []*RTSP
			collection.Range(func(key, value interface{}) bool {
				rtsp := value.(*RTSP)
				info = append(info, rtsp)
				return true
			})
			err = sse.WriteJSON(info)
		}
	})
	http.HandleFunc("/api/rtsp/pull", func(w http.ResponseWriter, r *http.Request) {
		CORS(w, r)
		targetURL := r.URL.Query().Get("target")
		streamPath := r.URL.Query().Get("streamPath")
		if err := new(RTSP).PullStream(streamPath, targetURL); err == nil {
			w.Write([]byte(`{"code":0}`))
		} else {
			w.Write([]byte(fmt.Sprintf(`{"code":1,"msg":"%s"}`, err.Error())))
		}
	})
	if len(config.AutoPullList) > 0 {
		for _, info := range config.AutoPullList {
			if err := new(RTSP).PullStream(info.StreamPath, info.URL); err != nil {
				Println(err)
			}
		}
	}
	if config.ListenAddr != "" {
		go log.Fatal(ListenRtsp(config.ListenAddr))
	}
	// AddHook(HOOK_SUBSCRIBE, func(value interface{}) {
	// 	s := value.(*Subscriber)
	// 	if config.AutoPull && s.Publisher == nil {
	// 		new(RTSP).PullStream(s.StreamPath, strings.Replace(config.RemoteAddr, "${streamPath}", s.StreamPath, -1))
	// 	}
	// })
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
	*Stream
	URL      string
	SDPRaw   string
	InBytes  int
	OutBytes int
	RTSPClientInfo
	ID        string
	Conn      *RichConn `json:"-"`
	connRW    *bufio.ReadWriter
	connWLock sync.RWMutex
	Type      SessionType
	TransType TransType

	SDPMap   map[string]*SDPInfo
	nonce    string
	closeOld bool
	ASdp     *SDPInfo
	VSdp     *SDPInfo
	aacsent  bool
	Timeout  int
	//tcp channels
	aRTPChannel        int
	aRTPControlChannel int
	vRTPChannel        int
	vRTPControlChannel int
	UDPServer          *UDPServer          `json:"-"`
	UDPClient          *UDPClient          `json:"-"`
	Auth               func(string) string `json:"-"`
	HasVideo           bool
	HasAudio           bool
	RtpAudio           *RTPAudio
	RtpVideo           *RTPVideo
}

func (rtsp *RTSP) setAudioFormat(at *AudioTrack) {
	switch rtsp.ASdp.Codec {
	case "aac":
		at.CodecID = 10
	case "pcma":
		at.CodecID = 7
		at.SoundRate = rtsp.ASdp.TimeScale
		at.SoundSize = 16
	case "pcmu":
		at.CodecID = 8
		at.SoundRate = rtsp.ASdp.TimeScale
		at.SoundSize = 16
	default:
		Printf("rtsp audio codec not support:%s", rtsp.ASdp.Codec)
		return
	}
	rtsp.AudioTracks.AddTrack(rtsp.ASdp.Codec, at)
}

type RTSPClientInfo struct {
	Agent    string
	Session  string
	authLine string
	Seq      int
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
