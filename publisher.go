package rtsp

import (
	"github.com/aler9/gortsplib/v2/pkg/codecs/mpeg4audio"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/pion/rtp"
	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/common"
	. "m7s.live/engine/v4/track"
)

type RTSPPublisher struct {
	Publisher
	Tracks map[*media.Media]common.AVTrack `json:"-"`
	RTSPIO
}

func (p *RTSPPublisher) SetTracks() error {
	p.Tracks = make(map[*media.Media]common.AVTrack, len(p.tracks))
	defer func() {
		for _, track := range p.Tracks {
			p.Info("set track", zap.String("name", track.GetBase().Name))
		}
	}()
	for _, track := range p.tracks {
		for _, forma := range track.Formats {
			switch f := forma.(type) {
			case *format.H264:
				vt := NewH264(p.Stream, f.PayloadType())
				p.Tracks[track] = vt
				if len(f.SPS) > 0 {
					vt.WriteSliceBytes(f.SPS)
				}
				if len(f.PPS) > 0 {
					vt.WriteSliceBytes(f.PPS)
				}
			case *format.H265:
				vt := NewH265(p.Stream, f.PayloadType())
				p.Tracks[track] = vt
				if len(f.VPS) > 0 {
					vt.WriteSliceBytes(f.VPS)
				}
				if len(f.SPS) > 0 {
					vt.WriteSliceBytes(f.SPS)
				}
				if len(f.PPS) > 0 {
					vt.WriteSliceBytes(f.PPS)
				}
			case *format.MPEG4Audio:
				at := NewAAC(p.Stream, f.PayloadType())
				p.Tracks[track] = at
				at.SizeLength = f.SizeLength
				if f.Config.Type == mpeg4audio.ObjectTypeAACLC {
					at.Mode = 1
				}
				at.SampleRate = uint32(f.Config.SampleRate)
				at.Channels = uint8(f.Config.ChannelCount)
				asc, _ := f.Config.Marshal()
				// 复用AVCC写入逻辑，解析出AAC的配置信息
				at.WriteSequenceHead(append([]byte{0xAF, 0x00}, asc...))
			case *format.G711:
				at := NewG711(p.Stream, !f.MULaw, f.PayloadType(), uint32(f.ClockRate()))
				p.Tracks[track] = at
				at.AVCCHead = []byte{(byte(at.CodecID) << 4) | (1 << 1)}
			}
		}
	}
	return nil
}

func (p *RTSPPublisher) OnPacket(m *media.Media, f format.Format, pack *rtp.Packet) {
	if t, ok := p.Tracks[m]; ok {
		t.WriteRTPPack(pack)
	}
}
