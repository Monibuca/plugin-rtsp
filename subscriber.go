package rtsp

import (
	"github.com/bluenviron/gortsplib/v3"
	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/bluenviron/gortsplib/v3/pkg/formats"
	"github.com/bluenviron/gortsplib/v3/pkg/media"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/codec"
	"m7s.live/engine/v4/track"
)

type RTSPSubscriber struct {
	Subscriber
	RTSPIO
}

func (s *RTSPSubscriber) OnEvent(event any) {
	switch v := event.(type) {
	case *track.Video:
		if s.Video != nil {
			return
		}
		switch v.CodecID {
		case codec.CodecID_H264:
			video := &media.Media{
				Type: media.TypeVideo,
				Formats: []formats.Format{&formats.H264{
					PacketizationMode: 1,
					PayloadTyp:        v.PayloadType,
					SPS:               v.ParamaterSets[0],
					PPS:               v.ParamaterSets[1],
				}},
			}
			s.videoTrack = video
			s.tracks = append(s.tracks, video)
		case codec.CodecID_H265:
			video := &media.Media{
				Type: media.TypeVideo,
				Formats: []formats.Format{&formats.H265{
					PayloadTyp: v.PayloadType,
					VPS:        v.ParamaterSets[0],
					SPS:        v.ParamaterSets[1],
					PPS:        v.ParamaterSets[2],
				}},
			}
			s.videoTrack = video
			s.tracks = append(s.tracks, video)
		}
		s.AddTrack(v)
	case *track.Audio:
		if s.Audio != nil {
			return
		}
		switch v.CodecID {
		case codec.CodecID_AAC:
			audio := &media.Media{
				Type: media.TypeAudio,
				Formats: []formats.Format{&formats.MPEG4Audio{
					PayloadTyp: v.PayloadType,
					Config: &mpeg4audio.Config{
						Type:         mpeg4audio.ObjectTypeAACLC,
						SampleRate:   int(v.SampleRate),
						ChannelCount: int(v.Channels),
					},
					SizeLength:       v.SizeLength,
					IndexLength:      v.IndexLength,
					IndexDeltaLength: v.IndexDeltaLength,
				}},
			}
			s.audioTrack = audio
			s.tracks = append(s.tracks, audio)
		case codec.CodecID_PCMA:
			audio := &media.Media{
				Type:    media.TypeAudio,
				Formats: []formats.Format{&formats.G711{}},
			}
			s.audioTrack = audio
			s.tracks = append(s.tracks, audio)
		case codec.CodecID_PCMU:
			audio := &media.Media{
				Type: media.TypeAudio,
				Formats: []formats.Format{&formats.G711{
					MULaw: true,
				}},
			}
			s.audioTrack = audio
			s.tracks = append(s.tracks, audio)
		}
		s.AddTrack(v)
	case ISubscriber:
		s.stream = gortsplib.NewServerStream(s.tracks)
	case VideoRTP:
		s.stream.WritePacketRTP(s.videoTrack, v.Packet)
	case AudioRTP:
		s.stream.WritePacketRTP(s.audioTrack, v.Packet)
	default:
		s.Subscriber.OnEvent(event)
	}
}
