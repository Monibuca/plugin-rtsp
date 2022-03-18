package rtsp

import (
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	. "m7s.live/engine/v4"

)

type RTSPIO struct {
	tracks       gortsplib.Tracks
	stream       *gortsplib.ServerStream
	audioTrackId int
	videoTrackId int
}

func (conf *RTSPConfig) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	plugin.Debug("conn opened")
}

func (conf *RTSPConfig) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	plugin.Debug("conn closed")
	if p, ok := conf.LoadAndDelete(ctx.Conn); ok {
		p.(IIO).Stop()
	}
}

func (conf *RTSPConfig) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	plugin.Debug("session opened")
}

func (conf *RTSPConfig) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	plugin.Debug("session closed")
	if p, ok := conf.LoadAndDelete(ctx.Session); ok {
		p.(IIO).Stop()
	}
}

// called after receiving a DESCRIBE request.
func (conf *RTSPConfig) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	plugin.Debug("describe request")
	var suber RTSPSubscriber
	if err := plugin.Subscribe(ctx.Path, &suber); err == nil {
		conf.Store(ctx.Conn, &suber)
		return &base.Response{
			StatusCode: base.StatusOK,
		}, suber.stream, nil
	} else {
		return nil, nil, err
	}
}

func (conf *RTSPConfig) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	var resp base.Response
	resp.StatusCode = base.StatusOK
	if p, ok := conf.Load(ctx.Conn); ok {
		switch v := p.(type) {
		case *RTSPSubscriber:
			return &resp, v.stream, nil
		}
	}
	resp.StatusCode = base.StatusNotFound
	return &resp, nil, nil
}

func (conf *RTSPConfig) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	var resp base.Response
	resp.StatusCode = base.StatusNotFound
	if p, ok := conf.Load(ctx.Conn); ok {
		switch v := p.(type) {
		case *RTSPSubscriber:
			resp.StatusCode = base.StatusOK
			go func() {
				v.PlayBlock(v)
				ctx.Conn.Close()
			}()
		}
	}
	return &resp, nil
}

func (conf *RTSPConfig) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	p := &RTSPPublisher{}
	if err := plugin.Publish(ctx.Path, p); err == nil {
		p.tracks = ctx.Tracks
		p.stream = gortsplib.NewServerStream(ctx.Tracks)
		p.SetTracks()

		conf.Store(ctx.Conn, p)
		conf.Store(ctx.Session, p)
	} else {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, err
	}
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (conf *RTSPConfig) OnPacketRTP(ctx *gortsplib.ServerHandlerOnPacketRTPCtx) {
	if p, ok := conf.Load(ctx.Session); ok {
		switch v := p.(type) {
		case *RTSPPublisher:
			v.Tracks[ctx.TrackID].WriteRTPPack(ctx.Packet)
		}
	}
}
