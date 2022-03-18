package rtsp

import (
	"sync"

	"github.com/aler9/gortsplib"
	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/config"
)

type RTSPConfig struct {
	config.Publish
	config.Subscribe
	config.Pull
	config.Push
	ListenAddr     string
	UDPAddr        string
	RTCPAddr       string
	ReadBufferSize int
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
		}
		s.Start()
		if conf.PullOnStart {
			for streamPath, url := range conf.PullList {
				if err := plugin.Pull(streamPath, url, new(RTSPPuller), false); err != nil {
					plugin.Error("pull", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
				}
			}
		}
	case SEpublish:
		for streamPath, url := range conf.PushList {
			if streamPath == v.Stream.Path {
				if err := plugin.Push(streamPath, url, new(RTSPPusher), false); err != nil {
					plugin.Error("push", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
				}
			}
		}
	case *Stream: //按需拉流
		if conf.PullOnSubscribe {
			for streamPath, url := range conf.PullList {
				if streamPath == v.Path {
					if err := plugin.Pull(streamPath, url, new(RTSPPuller), false); err != nil {
						plugin.Error("pull", zap.String("streamPath", streamPath), zap.String("url", url), zap.Error(err))
					}
					break
				}
			}
		}
	}
}

var rtspConfig = &RTSPConfig{
	ListenAddr:     ":554",
	UDPAddr:        ":8000",
	RTCPAddr:       ":8001",
	ReadBufferSize: 2048,
}
var plugin = InstallPlugin(rtspConfig)
