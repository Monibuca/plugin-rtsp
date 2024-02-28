package rtsp

import (
	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"go.uber.org/zap"
	"m7s.live/engine/v4/common"
)

type RTSPIO struct {
	server     *gortsplib.Server
	session    *description.Session
	tracks     []*description.Media
	stream     *gortsplib.ServerStream
	audioTrack *description.Media
	videoTrack *description.Media
}

func (conf *RTSPConfig) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	RTSPPlugin.Debug("conn opened")
}

func (conf *RTSPConfig) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	RTSPPlugin.Debug("conn closed")
	if p, ok := conf.LoadAndDelete(ctx.Conn); ok {
		p.(common.IIO).Stop(zap.String("conn", "closed"))
	}
}

func (conf *RTSPConfig) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	RTSPPlugin.Debug("session opened")
}

func (conf *RTSPConfig) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	RTSPPlugin.Debug("session closed")
	conf.Delete(ctx.Session)
}

// called after receiving a DESCRIBE request.
func (conf *RTSPConfig) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	RTSPPlugin.Debug("describe request", zap.String("sdp", string(ctx.Request.Body)))
	var suber RTSPSubscriber
	suber.server = conf.server
	suber.RemoteAddr = ctx.Conn.NetConn().RemoteAddr().String()
	suber.SetIO(ctx.Conn.NetConn())
	streamPath := ctx.Path
	if ctx.Query != "" {
		streamPath = streamPath + "?" + ctx.Query
	}
	if err := RTSPPlugin.Subscribe(streamPath, &suber); err == nil {
		RTSPPlugin.Debug("describe replay ok")
		conf.Store(ctx.Conn, &suber)
		return &base.Response{
			StatusCode: base.StatusOK,
		}, suber.stream, nil
	} else {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, suber.stream, nil
	}
}

func (conf *RTSPConfig) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	var resp base.Response
	resp.StatusCode = base.StatusOK
	if p, ok := conf.Load(ctx.Conn); ok {
		switch v := p.(type) {
		case *RTSPSubscriber:
			return &resp, v.stream, nil
		case *RTSPPublisher:
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
				v.PlayRTP()
				ctx.Session.Close()
			}()
		}
	}
	return &resp, nil
}
func (conf *RTSPConfig) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	if p, ok := conf.Load(ctx.Session); ok {
		ctx.Session.OnPacketRTPAny(p.(*RTSPPublisher).OnPacket)
	}
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}
func (conf *RTSPConfig) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	RTSPPlugin.Debug("annouce request", zap.String("sdp", string(ctx.Request.Body)))
	p := &RTSPPublisher{}
	p.SetIO(ctx.Conn.NetConn())
	if err := RTSPPlugin.Publish(ctx.Path, p); err == nil {
		p.session = ctx.Description
		p.stream = gortsplib.NewServerStream(conf.server, ctx.Description)
		if err = p.SetTracks(); err != nil {
			return nil, err
		}
		conf.Store(ctx.Conn, p)
		conf.Store(ctx.Session, p)
	} else {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, nil
	}
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}
