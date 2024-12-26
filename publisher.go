package rtsp

import (
	"strings"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"
	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/codec"
	"m7s.live/engine/v4/common"
	. "m7s.live/engine/v4/track"
	"m7s.live/engine/v4/util"
)

type RTSPPublisher struct {
	Publisher
	Tracks map[*description.Media]common.AVTrack `json:"-" yaml:"-"`
	RTSPIO
}

func (p *RTSPPublisher) SetTracks() error {
	p.Tracks = make(map[*description.Media]common.AVTrack, len(p.session.Medias))
	defer func() {
		for _, track := range p.Tracks {
			p.Info("set track", zap.String("name", track.GetName()))
		}
	}()
	for _, track := range p.session.Medias {
		for _, forma := range track.Formats {
			switch f := forma.(type) {
			case *format.H264:
				vt := p.VideoTrack
				if vt == nil {
					vt = p.CreateVideoTrack(codec.CodecID_H264, byte(f.PayloadType()))
				}
				p.Tracks[track] = vt
				if len(f.SPS) > 0 {
					vt.WriteSliceBytes(f.SPS)
				}
				if len(f.PPS) > 0 {
					vt.WriteSliceBytes(f.PPS)
				}
			case *format.H265:
				vt := p.VideoTrack
				if vt == nil {
					vt = p.CreateVideoTrack(codec.CodecID_H265, byte(f.PayloadType()))
				}
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
			case *format.AV1:
				vt := p.VideoTrack
				if vt == nil {
					vt = p.CreateVideoTrack(codec.CodecID_AV1, byte(f.PayloadType()))
				}
				p.Tracks[track] = vt
			case *format.MPEG4Audio:
				at := p.AudioTrack
				if at == nil {
					conf := f.Config
					if f.LATM && f.StreamMuxConfig != nil && len(f.StreamMuxConfig.Programs) > 0 && len(f.StreamMuxConfig.Programs[0].Layers) > 0 {
						conf = f.StreamMuxConfig.Programs[0].Layers[0].AudioSpecificConfig
					}
					at := p.CreateAudioTrack(codec.CodecID_AAC, byte(f.PayloadType()), uint32(conf.SampleRate)).(*AAC)
					at.AACFormat = f
					at.AACDecoder.LATM = f.LATM
					at.AACDecoder.IndexDeltaLength = f.IndexDeltaLength
					at.AACDecoder.IndexLength = f.IndexLength
					at.AACDecoder.SizeLength = f.SizeLength
					if conf.Type == mpeg4audio.ObjectTypeAACLC {
						at.Mode = 1
					}
					at.Channels = uint8(conf.ChannelCount)
					asc, _ := conf.Marshal()
					// 复用AVCC写入逻辑，解析出AAC的配置信息
					at.WriteSequenceHead(append([]byte{0xAF, 0x00}, asc...))
				}
				p.Tracks[track] = p.AudioTrack
			case *format.G711:
				at := p.AudioTrack
				if at == nil {
					at = p.CreateAudioTrack(util.Conditoinal(f.MULaw, codec.CodecID_PCMU, codec.CodecID_PCMA), byte(f.PayloadType()), uint32(f.ClockRate()))
				}
				p.Tracks[track] = at
			case *format.Opus:
				at := p.AudioTrack
				if at == nil {
					at = p.CreateAudioTrack(codec.CodecID_OPUS, byte(f.PayloadType()), uint32(f.ClockRate()))
				}
				p.Tracks[track] = at
			default:
				rtpMap := strings.ToLower(forma.RTPMap())
				if strings.Contains(rtpMap, "pcm") {
					isMulaw := false
					if strings.Contains(rtpMap, "pcmu") {
						isMulaw = true
					}
					at := p.AudioTrack
					if at == nil {
						at = p.CreateAudioTrack(util.Conditoinal(isMulaw, codec.CodecID_PCMU, codec.CodecID_PCMA), byte(f.PayloadType()), uint32(f.ClockRate()))
					}
					p.Tracks[track] = at
				} else {
					p.Warn("unknown format", zap.Any("format", f.FMTP()))
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

func (p *RTSPPublisher) OnPacket(m *description.Media, f format.Format, pack *rtp.Packet) {
	if t, ok := p.Tracks[m]; ok {
		t.WriteRTPPack(pack)
	}
}
