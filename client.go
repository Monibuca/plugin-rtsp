package rtsp

import (
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
	"m7s.live/engine/v4"
)

type RTSPPuller struct {
	RTSPPublisher
	engine.Puller
	*gortsplib.Client `json:"-"`
	gortsplib.Transport
}

func (p *RTSPPuller) Connect() error {
	switch rtspConfig.PullProtocol {
	case "tcp", "TCP":
		p.Transport = gortsplib.TransportTCP
	case "udp", "UDP":
		p.Transport = gortsplib.TransportUDP
	default:
		if p.Transport == gortsplib.TransportTCP {
			p.Transport = gortsplib.TransportUDP
		} else {
			p.Transport = gortsplib.TransportTCP
		}
	}
	p.Client = &gortsplib.Client{
		OnPacketRTP: func(ctx *gortsplib.ClientOnPacketRTPCtx) {
			if p.RTSPPublisher.Tracks[ctx.TrackID] != nil {
				p.RTSPPublisher.Tracks[ctx.TrackID].WriteRTPPack(ctx.Packet)
			}
		},
		ReadBufferCount: rtspConfig.ReadBufferSize,
		Transport:       &p.Transport,
	}
	// parse URL
	u, err := url.Parse(p.RemoteURL)
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
	u, _ := url.Parse(p.RemoteURL)
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
	case engine.VideoRTP:
		p.Client.WritePacketRTP(p.videoTrackId, &v.Packet, p.Video.Frame.PTS == p.Video.Frame.DTS)
	case engine.AudioRTP:
		p.Client.WritePacketRTP(p.audioTrackId, &v.Packet, p.Audio.Frame.PTS == p.Audio.Frame.DTS)
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
		ReadBufferCount: rtspConfig.ReadBufferSize,
		Transport:       &p.Transport,
	}
	// parse URL
	u, err := url.Parse(p.RemoteURL)
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
	var u *url.URL
	u, err = url.Parse(p.RemoteURL)
	defer func() {
		if err != nil {
			p.Close()
		}
	}()
	// startTime := time.Now()
	// for len(p.tracks) < 2 {
	// 	if time.Sleep(time.Second); time.Since(startTime) > time.Second*10 {
	// 		return fmt.Errorf("timeout")
	// 	}
	// }
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
	p.PlayRTP()
	return
}
