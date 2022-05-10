package rtsp

import (
	"fmt"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/pion/rtp/v2"
	"m7s.live/engine/v4"
)

type RTSPPuller struct {
	RTSPPublisher
	engine.Puller
	*gortsplib.Client
	gortsplib.Transport
}

func (p *RTSPPuller) Connect() error {
	if p.Transport == gortsplib.TransportTCP {
		p.Transport = gortsplib.TransportUDP
	} else {
		p.Transport = gortsplib.TransportTCP
	}
	p.Client = &gortsplib.Client{
		OnPacketRTP: func(trackID int, packet *rtp.Packet) {
			p.RTSPPublisher.Tracks[trackID].WriteRTPPack(packet)
		},
		ReadBufferSize: rtspConfig.ReadBufferSize,
		Transport:      &p.Transport,
	}
	// parse URL
	u, err := base.ParseURL(p.RemoteURL)
	if err != nil {
		return err
	}
	// connect to the server
	if err = p.Client.Start(u.Scheme, u.Host); err != nil {
		return err
	}
	return nil
}

func (p *RTSPPuller) Pull() {
	u, _ := base.ParseURL(p.RemoteURL)
	if _, err := p.Options(u); err != nil {
		return
	}
	// find published tracks
	tracks, baseURL, _, err := p.Describe(u)
	if err != nil {
		return
	}
	p.tracks = tracks
	p.SetTracks()
	if err = p.SetupAndPlay(tracks, baseURL); err != nil {
		return
	}
	p.Wait()
}

type RTSPPusher struct {
	RTSPSubscriber
	engine.Pusher
	*gortsplib.Client
	gortsplib.Transport
}

func (p *RTSPPusher) OnEvent(event any) {
	switch v := event.(type) {
	case *engine.AudioFrame:
		for _, pack := range v.RTP {
			p.Client.WritePacketRTP(p.audioTrackId, &pack.Packet)
		}
	case *engine.VideoFrame:
		for _, pack := range v.RTP {
			p.Client.WritePacketRTP(p.videoTrackId, &pack.Packet)
		}
	default:
		p.RTSPSubscriber.OnEvent(event)
	}
}
func (p *RTSPPusher) Connect() error {
	if p.Transport == gortsplib.TransportTCP {
		p.Transport = gortsplib.TransportUDP
	} else {
		p.Transport = gortsplib.TransportTCP
	}
	p.Client = &gortsplib.Client{
		ReadBufferSize: rtspConfig.ReadBufferSize,
		Transport:      &p.Transport,
	}
	// parse URL
	u, err := base.ParseURL(p.RemoteURL)
	if err != nil {
		return err
	}
	// connect to the server
	if err = p.Client.Start(u.Scheme, u.Host); err != nil {
		return err
	}
	_, err = p.Client.Options(u)
	return err
}
func (p *RTSPPusher) Push() (err error) {
	var u *base.URL
	u, err = base.ParseURL(p.RemoteURL)
	defer func() {
		if err != nil {
			p.Close()
		}
	}()
	startTime := time.Now()
	for len(p.tracks) < 2 {
		if time.Sleep(time.Second); time.Since(startTime) > time.Second*10 {
			return fmt.Errorf("timeout")
		}
	}
	if _, err = p.Announce(u, p.tracks); err != nil {
		return
	}
	for _, track := range p.tracks {
		_, err = p.Setup(false, track, u, 0, 0)
		if err != nil {
			return
		}
	}
	if _, err = p.Record(); err != nil {
		return
	}
	p.PlayBlock()
	return
}
