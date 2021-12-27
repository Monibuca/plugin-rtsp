package rtsp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"github.com/aler9/gortsplib"
)

var config = struct {
	ListenAddr   string
	UDPAddr      string
	RTCPAddr     string
	Timeout      int
	Reconnect    bool
	AutoPullList map[string]string
	AutoPushList map[string]string
}{":554", ":8000", ":8001", 0, false, nil, nil}

var pconfig = PluginConfig{
	Name:   "RTSP",
	Config: &config,
}

func init() {
	pconfig.Install(runPlugin)
}

func getRtspList() (info []*Stream) {
	for _, s := range Streams.ToList() {
		switch s.ExtraProp.(type) {
		case *RTSPublisher:
			info = append(info, s)
		case *RTSPClient:
			info = append(info, s)
		}
	}
	return
}
func runPlugin() {
	http.HandleFunc("/api/rtsp/list", func(w http.ResponseWriter, r *http.Request) {
		CORS(w, r)
		if r.URL.Query().Get("json") != "" {
			if jsonData, err := json.Marshal(getRtspList()); err == nil {
				w.Write(jsonData)
			} else {
				w.WriteHeader(500)
			}
			return
		}
		sse := NewSSE(w, r.Context())
		var err error
		for tick := time.NewTicker(time.Second); err == nil; <-tick.C {
			err = sse.WriteJSON(getRtspList())
		}
	})
	http.HandleFunc("/api/rtsp/pull", func(w http.ResponseWriter, r *http.Request) {
		CORS(w, r)
		targetURL := r.URL.Query().Get("target")
		streamPath := r.URL.Query().Get("streamPath")
		save := r.URL.Query().Get("save")
		if err := (&RTSPClient{Transport: gortsplib.TransportTCP}).PullStream(streamPath, targetURL); err == nil {
			if save == "1" {
				if config.AutoPullList == nil {
					config.AutoPullList = make(map[string]string)
				}
				config.AutoPullList[streamPath] = targetURL
				if err := pconfig.Save(); err != nil {
					Println(err)
				}
			}
			w.Write([]byte(`{"code":0}`))
		} else {
			w.Write([]byte(fmt.Sprintf(`{"code":1,"msg":"%s"}`, err.Error())))
		}
	})
	for streamPath, url := range config.AutoPullList {
		if err := (&RTSPClient{Transport: gortsplib.TransportTCP}).PullStream(streamPath, url); err != nil {
			Println(err)
		}
	}
	
	go AddHook(HOOK_PUBLISH, func(s *Stream) {
		for streamPath, url := range config.AutoPushList {
			if s.StreamPath == streamPath {
				(&RTSPClient{}).PushStream(streamPath, url)
			}
		}
	})
	if config.ListenAddr != "" {
		go log.Fatal(ListenRtsp(config.ListenAddr))
	}
}

func ListenRtsp(addr string) error {
	defer log.Println("rtsp server start!")
	s := &gortsplib.Server{
		Handler:           &RTSPServer{},
		RTSPAddress:       addr,
		UDPRTPAddress:     config.UDPAddr,
		UDPRTCPAddress:    config.RTCPAddr,
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}
	return s.StartAndWait()
}
