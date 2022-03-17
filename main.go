package rtsp

import (
	"sync"

	"github.com/aler9/gortsplib"
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
	switch event.(type) {
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
	}
}

var plugin = InstallPlugin(&RTSPConfig{
	ListenAddr:     ":554",
	UDPAddr:        ":8000",
	RTCPAddr:       ":8001",
	ReadBufferSize: 2048,
})
