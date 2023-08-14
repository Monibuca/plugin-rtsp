package rtsp

import (
	"strings"

	"github.com/bluenviron/gortsplib/v3/pkg/formats"
	"github.com/bluenviron/gortsplib/v3/pkg/media"
	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"
	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/common"
	. "m7s.live/engine/v4/track"
)

type RTSPPublisher struct {
	Publisher
	Tracks map[*media.Media]common.AVTrack `json:"-" yaml:"-"`
	RTSPIO
}

func (p *RTSPPublisher) SetTracks() error {
	p.Tracks = make(map[*media.Media]common.AVTrack, len(p.tracks))
	defer func() {
		for _, track := range p.Tracks {
			p.Info("set track", zap.String("name", track.GetName()))
		}
	}()
	for _, track := range p.tracks {
		for _, forma := range track.Formats {
			switch f := forma.(type) {
			case *formats.H264:
				vt := p.VideoTrack
				if vt == nil {
					vt = NewH264(p.Stream, f.PayloadType())
					p.VideoTrack = vt
				}
				p.Tracks[track] = p.VideoTrack
				if len(f.SPS) > 0 {
					vt.WriteSliceBytes(f.SPS)
				}
				if len(f.PPS) > 0 {
					vt.WriteSliceBytes(f.PPS)
				}
			case *formats.H265:
				vt := p.VideoTrack
				if vt == nil {
					vt = NewH265(p.Stream, f.PayloadType())
					p.VideoTrack = vt
				}
				p.Tracks[track] = p.VideoTrack
				if len(f.VPS) > 0 {
					vt.WriteSliceBytes(f.VPS)
				}
				if len(f.SPS) > 0 {
					vt.WriteSliceBytes(f.SPS)
				}
				if len(f.PPS) > 0 {
					vt.WriteSliceBytes(f.PPS)
				}
			case *formats.MPEG4Audio:
				at := p.AudioTrack
				if at == nil {
					at := NewAAC(p.Stream, f.PayloadType(), uint32(f.Config.SampleRate))
					at.IndexDeltaLength = f.IndexDeltaLength
					at.IndexLength = f.IndexLength
					at.SizeLength = f.SizeLength
					if f.Config.Type == mpeg4audio.ObjectTypeAACLC {
						at.Mode = 1
					}
					at.Channels = uint8(f.Config.ChannelCount)
					asc, _ := f.Config.Marshal()
					// 复用AVCC写入逻辑，解析出AAC的配置信息
					at.WriteSequenceHead(append([]byte{0xAF, 0x00}, asc...))
					p.AudioTrack = at
				}
				p.Tracks[track] = p.AudioTrack
			case *formats.G711:
				at := p.AudioTrack
				if at == nil {
					at := NewG711(p.Stream, !f.MULaw, f.PayloadType(), uint32(f.ClockRate()))
					p.AudioTrack = at
				}
				p.Tracks[track] = p.AudioTrack
			default:
				rtpMap := strings.ToLower(forma.RTPMap())
				if strings.Contains(rtpMap, "pcm") {
					isMulaw := false
					if strings.Contains(rtpMap, "pcmu") {
						isMulaw = true
					}
					at := p.AudioTrack
					if at == nil {
						at := NewG711(p.Stream, !isMulaw, f.PayloadType(), uint32(f.ClockRate()))
						p.AudioTrack = at
					}
					p.Tracks[track] = p.AudioTrack
				} else {
					p.Warn("unknown format", zap.Any("format", f.Codec()))
				}
			}
		}
	}
	if p.VideoTrack == nil {
		p.Config.PubVideo = false
		p.Info("no video track")
	}
	if p.AudioTrack == nil {
		p.Config.PubAudio = false
		p.Info("no audio track")
	}
	return nil
}

func (p *RTSPPublisher) OnPacket(m *media.Media, f formats.Format, pack *rtp.Packet) {
	if t, ok := p.Tracks[m]; ok {
		t.WriteRTPPack(pack)
	}
}
