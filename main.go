package rtsp

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v3"
	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/config"
	"m7s.live/engine/v4/util"
)

type RTSPConfig struct {
	config.Publish
	config.Subscribe
	config.Pull
	config.Push
	ListenAddr       string `default:":554"`
	UDPAddr          string `default:":8000"`
	RTCPAddr         string `default:":8001"`
	ReadBufferCount  int    `default:"2048"`
	WriteBufferCount int    `default:"2048"`
	PullProtocol     string `default:"tcp"` //tcp、udp、auto
	sync.Map
}

func (conf *RTSPConfig) OnEvent(event any) {
	switch v := event.(type) {
	case FirstConfig:
		s := &gortsplib.Server{
			Handler:           conf,
			RTSPAddress:       conf.ListenAddr,
			UDPRTPAddress:     conf.UDPAddr,
			UDPRTCPAddress:    conf.RTCPAddr,
			MulticastIPRange:  "224.1.0.0/16",
			MulticastRTPPort:  8002,
			MulticastRTCPPort: 8003,
			ReadBufferCount:   conf.ReadBufferCount,
			WriteBufferCount:  conf.WriteBufferCount,
		}
		if err := s.Start(); err != nil {
			RTSPPlugin.Error("server start", zap.Error(err))
			RTSPPlugin.Disabled = true
		}
		for streamPath, url := range conf.PullOnStart {
			if err := RTSPPlugin.Pull(streamPath, url, new(RTSPPuller), 0); err != nil {
				RTSPPlugin.Error("pull", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
			}
		}
	case SEpublish:
		if url, ok := conf.PushList[v.Target.Path]; ok {
			if err := RTSPPlugin.Push(v.Target.Path, url, new(RTSPPusher), false); err != nil {
				RTSPPlugin.Error("push", zap.String("streamPath", v.Target.Path), zap.String("url", url), zap.Error(err))
			}
		}
	case *Stream: //按需拉流
		if url, ok := conf.PullOnSub[v.Path]; ok {
			if err := RTSPPlugin.Pull(v.Path, url, new(RTSPPuller), 0); err != nil {
				RTSPPlugin.Error("pull", zap.String("streamPath", v.Path), zap.String("url", url), zap.Error(err))
			}
		}
	}
}

var rtspConfig = &RTSPConfig{}
var RTSPPlugin = InstallPlugin(rtspConfig)

func filterStreams() (ss []*Stream) {
	Streams.Range(func(key string, s *Stream) {
		switch s.Publisher.(type) {
		case *RTSPPublisher, *RTSPPuller:
			ss = append(ss, s)
		}
	})
	return
}

func (*RTSPConfig) API_list(w http.ResponseWriter, r *http.Request) {
	util.ReturnJson(filterStreams, time.Second, w, r)
}

func (*RTSPConfig) API_Pull(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	save, _ := strconv.Atoi(query.Get("save"))
	err := RTSPPlugin.Pull(query.Get("streamPath"), query.Get("target"), new(RTSPPuller), save)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
	} else {
		rw.Write([]byte("ok"))
	}
}

func (*RTSPConfig) API_Push(rw http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	err := RTSPPlugin.Push(query.Get("streamPath"), query.Get("target"), new(RTSPPusher), query.Has("save"))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
	} else {
		rw.Write([]byte("ok"))
	}
}
