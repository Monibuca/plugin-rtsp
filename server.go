package rtsp

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"github.com/Monibuca/utils/v3/codec"
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/aac"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
)

// 接收RTSP推流：OnConnOpen->OnAnnounce->OnSetup->OnSessionOpen
// 接收RTSP拉流：OnConnOpen->OnDescribe->OnSetup->OnSessionOpen
type RTSPServer struct {
	sync.Map
}
type RTSPSubscriber struct {
	stream *gortsplib.ServerStream
	engine.Subscriber
	vt *engine.VideoTrack
	at *engine.AudioTrack
}

// called after a connection is opened.
func (sh *RTSPServer) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	Printf("rtsp conn opened")
}

// called after a connection is closed.
func (sh *RTSPServer) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	Printf("rtsp conn closed (%v)", ctx.Error)
	if p, ok := sh.Load(ctx.Conn); ok {
		switch v := p.(type) {
		case *RTSPublisher:
			v.Close()
		case *RTSPSubscriber:
			v.Close()
		}
		sh.Delete(ctx.Conn)
	}
}

// called after a session is opened.
func (sh *RTSPServer) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	Printf("rtsp session opened")
}

// called after a session is closed.
func (sh *RTSPServer) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	Printf("rtsp session closed")
	if v, ok := sh.LoadAndDelete(ctx.Session); ok {
		switch v := v.(type) {
		case *RTSPublisher:
			v.Close()
		case *RTSPSubscriber:
			v.Close()
		}
	}
}

// called after receiving a DESCRIBE request.
func (sh *RTSPServer) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	Printf("describe request")
	var err error
	if s := engine.FindStream(ctx.Path); s != nil {
		var tracks gortsplib.Tracks
		var stream *gortsplib.ServerStream
		var sub RTSPSubscriber
		sub.Type = "RTSP pull"
		sub.vt = s.WaitVideoTrack("h264", "h265")
		sub.at = s.WaitAudioTrack("aac", "pcma", "pcmu")
		ssrc := uintptr(unsafe.Pointer(stream))
		var trackIds = 0
		if sub.vt != nil {
			trackId := trackIds
			var vtrack *gortsplib.Track
			var vpacketer rtp.Packetizer
			switch sub.vt.CodecID {
			case codec.CodecID_H264:
				if vtrack, err = gortsplib.NewTrackH264(96, &gortsplib.TrackConfigH264{
					SPS: sub.vt.ExtraData.NALUs[0],
					PPS: sub.vt.ExtraData.NALUs[1],
				}); err == nil {
					vpacketer = rtp.NewPacketizer(1200, 96, uint32(ssrc), &codecs.H264Payloader{}, rtp.NewFixedSequencer(1), 90000)
				} else {
					return nil, nil, err
				}
			case codec.CodecID_H265:
				vtrack = NewH265Track(96, sub.vt.ExtraData.NALUs)
				vpacketer = rtp.NewPacketizer(1200, 96, uint32(ssrc), &H265Payloader{}, rtp.NewFixedSequencer(1), 90000)
			}
			var st uint32
			onVideo := func(ts uint32, pack *engine.VideoPack) {
				if pack.IDR {
					for _, nalu := range sub.vt.ExtraData.NALUs {
						for _, packet := range vpacketer.Packetize(nalu, 0) {
							buf, _ := packet.Marshal()
							stream.WritePacketRTP(trackId, buf)
						}
					}
				}
				for i, nalu := range pack.NALUs {
					var samples uint32
					if i == len(pack.NALUs)-1 {
						samples = (ts-st)*90
					} else {
						samples = 0
					}
					packs := vpacketer.Packetize(nalu, samples)
					for j, rtpack := range packs {
						rtpack.Marker = i == len(pack.NALUs)-1 && j == len(packs)-1
						rtp, _ := rtpack.Marshal()
						stream.WritePacketRTP(trackId, rtp)
					}
				}
				st = ts
			}
			sub.OnVideo = func(ts uint32, pack *engine.VideoPack) {
				if st = ts; st != 0 {
					sub.OnVideo = onVideo
				}
				onVideo(ts, pack)
			}
			tracks = append(tracks, vtrack)
			trackIds++
		}
		if sub.at != nil {
			var st uint32
			trackId := trackIds
			switch sub.at.CodecID {
			case codec.CodecID_PCMA, codec.CodecID_PCMU:
				atrack := NewG711Track(97, map[byte]string{7: "pcma", 8: "pcmu"}[sub.at.CodecID])
				apacketizer := rtp.NewPacketizer(1200, 97, uint32(ssrc), &codecs.G711Payloader{}, rtp.NewFixedSequencer(1), 8000)
				sub.OnAudio = func(ts uint32, pack *engine.AudioPack) {
					for _, pack := range apacketizer.Packetize(pack.Raw, (ts-st)*8) {
						buf, _ := pack.Marshal()
						stream.WritePacketRTP(trackId, buf)
					}
					st = ts
				}
				tracks = append(tracks, atrack)
			case codec.CodecID_AAC:
				var mpegConf aac.MPEG4AudioConfig
				mpegConf.Decode(sub.at.ExtraData[2:])
				conf := &gortsplib.TrackConfigAAC{
					Type:              int(mpegConf.Type),
					SampleRate:        mpegConf.SampleRate,
					ChannelCount:      mpegConf.ChannelCount,
					AOTSpecificConfig: mpegConf.AOTSpecificConfig,
				}
				if atrack, err := gortsplib.NewTrackAAC(97, conf); err == nil {
					apacketizer := rtp.NewPacketizer(1200, 97, uint32(ssrc), &AACPayloader{}, rtp.NewFixedSequencer(1), uint32(mpegConf.SampleRate))
					sub.OnAudio = func(ts uint32, pack *engine.AudioPack) {
						for _, pack := range apacketizer.Packetize(pack.Raw, (ts-st)*uint32(mpegConf.SampleRate)/1000) {
							buf, _ := pack.Marshal()
							stream.WritePacketRTP(trackId, buf)
						}
						st = ts
					}
					tracks = append(tracks, atrack)
				}
			}
		}
		stream = gortsplib.NewServerStream(tracks)
		sub.stream = stream
		sh.Store(ctx.Conn, &sub)
		return &base.Response{
			StatusCode: base.StatusOK,
		}, stream, nil
		// if stream, ok := s.ExtraProp.(*gortsplib.ServerStream); ok {
		// 	return &base.Response{
		// 		StatusCode: base.StatusOK,
		// 	}, stream, nil
		// }
	}
	return &base.Response{
		StatusCode: base.StatusNotFound,
	}, nil, nil
}

// called after receiving an ANNOUNCE request.
func (sh *RTSPServer) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	Printf("announce request")
	p := &RTSPublisher{
		Stream: &engine.Stream{
			StreamPath: ctx.Path,
			Type:       "RTSP push",
		},
	}
	p.ExtraProp = p
	p.URL = ctx.Req.URL.String()
	if p.Publish() {
		p.setTracks(ctx.Tracks)
		p.stream = gortsplib.NewServerStream(ctx.Tracks)
		sh.Store(ctx.Conn, p)
		sh.Store(ctx.Session, p)
	} else {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("streamPath is already exist")
	}
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a SETUP request.
func (sh *RTSPServer) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	Printf("setup request")
	if p, ok := sh.Load(ctx.Conn); ok {
		switch v := p.(type) {
		case *RTSPublisher:
			return &base.Response{
				StatusCode: base.StatusOK,
			}, v.stream, nil
		case *RTSPSubscriber:
			return &base.Response{
				StatusCode: base.StatusOK,
			}, v.stream, nil
		}
	}
	return &base.Response{
		StatusCode: base.StatusNotFound,
	}, nil, nil
}

// called after receiving a PLAY request.
func (sh *RTSPServer) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	Printf("play request")
	if p, ok := sh.Load(ctx.Conn); ok {
		if sub := p.(*RTSPSubscriber); sub.Subscribe(ctx.Path) == nil {
			go func() {
				sub.Play(sub.at, sub.vt)
				ctx.Conn.Close()
			}()
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}
	}
	return &base.Response{
		StatusCode: base.StatusNotFound,
	}, nil
}

// called after receiving a RECORD request.
func (sh *RTSPServer) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	Printf("record request")
	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a frame.
func (sh *RTSPServer) OnPacketRTP(ctx *gortsplib.ServerHandlerOnPacketRTPCtx) {
	if p, ok := sh.Load(ctx.Session); ok {
		rtsp := p.(*RTSPublisher)
		if rtsp.Err() != nil {
			ctx.Session.Close()
			return
		}
		if f := rtsp.processFunc[ctx.TrackID]; f != nil {
			f(ctx.Payload)
		}
	}
}
