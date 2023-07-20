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
	PullProtocol     string //tcp、udp、 auto（default）
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
	func (*RTSPConfig) API_Pull(rw http.ResponseWriter, r *http.Request) {
		save, _ := strconv.Atoi(r.URL.Query().Get("save"))
		// 截取原始请求参数，处理类似大华摄像头RTSP协议请求中【rtsp://user:passwd@ip:port/cam/realmonitor?channel=1&subtype=0】路径中存在&subtype，被误识别为参数，导致拉流失败的问题；
		rawQuery := r.URL.RawQuery
		start := strings.Index(rawQuery, "target=")
		if start == -1 {
			http.Error(rw, "Missing target parameter", http.StatusBadRequest)
			return
		}
		end := strings.Index(rawQuery, "&streamPath=")
		if end == -1 {
			end = len(rawQuery)
		}
		target := rawQuery[start+len("target=") : end]
		err := RTSPPlugin.Pull(r.URL.Query().Get("streamPath"), target, new(RTSPPuller), save)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
		} else {
			rw.Write([]byte("ok"))
		}
	}
}

func (*RTSPConfig) API_Push(rw http.ResponseWriter, r *http.Request) {
	err := RTSPPlugin.Push(r.URL.Query().Get("streamPath"), r.URL.Query().Get("target"), new(RTSPPusher), r.URL.Query().Has("save"))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
	} else {
		rw.Write([]byte("ok"))
	}
}
