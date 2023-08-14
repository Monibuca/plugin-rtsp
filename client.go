package rtsp

import (
	"context"
	"net"

	"github.com/bluenviron/gortsplib/v3"
	"github.com/bluenviron/gortsplib/v3/pkg/url"
	"go.uber.org/zap"
	"m7s.live/engine/v4"
)

type RTSPClient struct {
	*gortsplib.Client `json:"-" yaml:"-"`
	gortsplib.Transport
	DialContext func(ctx context.Context, network, address string) (net.Conn, error) `json:"-" yaml:"-"`
}
type RTSPPuller struct {
	RTSPPublisher
	engine.Puller
	RTSPClient
}

func (p *RTSPClient) Disconnect() {
	if p.Client != nil {
		p.Client.Close()
	}
}

func (p *RTSPPuller) Connect() error {
	p.Client = &gortsplib.Client{
		DialContext:     p.DialContext,
		ReadBufferCount: rtspConfig.ReadBufferCount,
		AnyPortEnable:   true,
	}

	switch rtspConfig.PullProtocol {
	case "tcp", "TCP":
		p.Transport = gortsplib.TransportTCP
		p.Client.Transport = &p.Transport
	case "udp", "UDP":
		p.Transport = gortsplib.TransportUDP
		p.Client.Transport = &p.Transport
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
	p.SetIO(p.Client)
	return nil
}

func (p *RTSPPuller) Pull() (err error) {
	u, _ := url.Parse(p.RemoteURL)
	if _, err = p.Options(u); err != nil {
		p.Error("Options", zap.Error(err))
		return
	}
	// find published tracks
	tracks, baseURL, _, err := p.Describe(u)
	if err != nil {
		p.Error("Describe", zap.Error(err))
		return
	}
	p.tracks = tracks
	err = p.SetTracks()
	if err != nil {
		p.Error("SetTracks", zap.Error(err))
		return
	}
	if err = p.SetupAll(tracks, baseURL); err != nil {
		p.Error("SetupAndPlay", zap.Error(err))
		return
	}
	p.OnPacketRTPAny(p.OnPacket)
	_, err = p.Play(nil)
	if err != nil {
		p.Error("Play", zap.Error(err))
		return
	}
	return p.Wait()
}

type RTSPPusher struct {
	RTSPSubscriber
	engine.Pusher
	RTSPClient
}

func (p *RTSPPusher) OnEvent(event any) {
	switch v := event.(type) {
	case engine.VideoRTP:
		p.Client.WritePacketRTP(p.videoTrack, v.Packet)
	case engine.AudioRTP:
		p.Client.WritePacketRTP(p.audioTrack, v.Packet)
	default:
		p.RTSPSubscriber.OnEvent(event)
	}
}

func (p *RTSPPusher) Connect() error {
	p.Client = &gortsplib.Client{
		DialContext:      p.DialContext,
		WriteBufferCount: rtspConfig.WriteBufferCount,
	}
	// parse URL
	u, err := url.Parse(p.RemoteURL)
	if err != nil {
		p.Error("url.Parse", zap.Error(err))
		return err
	}
	// connect to the server
	if err = p.Client.Start(u.Scheme, u.Host); err != nil {
		p.Error("Client.Start", zap.Error(err))
		return err
	}
	p.SetIO(p.Client)
	_, err = p.Client.Options(u)
	return err
}
func (p *RTSPPusher) Push() (err error) {
	var u *url.URL
	u, err = url.Parse(p.RemoteURL)
	// startTime := time.Now()
	// for len(p.tracks) < 2 {
	// 	if time.Sleep(time.Second); time.Since(startTime) > time.Second*10 {
	// 		return fmt.Errorf("timeout")
	// 	}
	// }
	if _, err = p.Announce(u, p.tracks); err != nil {
		p.Error("Announce", zap.Error(err))
		return
	}
	err = p.SetupAll(p.tracks, u)
	if err != nil {
		p.Error("Setup", zap.Error(err))
		return
	}

	if _, err = p.Record(); err != nil {
		p.Error("Record", zap.Error(err))
		return
	}
	p.PlayRTP()
	return
}
