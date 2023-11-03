package rtsp

import (
	"fmt"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
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
			video := &description.Media{
				Type: description.MediaTypeVideo,
				Formats: []format.Format{&format.H264{
					PacketizationMode: 1,
					PayloadTyp:        v.PayloadType,
					SPS:               v.ParamaterSets[0],
					PPS:               v.ParamaterSets[1],
				}},
			}
			s.videoTrack = video
			s.tracks = append(s.tracks, video)
		case codec.CodecID_H265:
			video := &description.Media{
				Type: description.MediaTypeVideo,
				Formats: []format.Format{&format.H265{
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
			audio := &description.Media{
				Type: description.MediaTypeAudio,
				Formats: []format.Format{&format.MPEG4Audio{
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
			audio := &description.Media{
				Type: description.MediaTypeAudio,
				Formats: []format.Format{&format.Generic{
					PayloadTyp: v.PayloadType,
					ClockRat:   int(v.SampleRate),
					RTPMa:      fmt.Sprintf("PCMA/%d", v.SampleRate),
				}},
			}
			s.audioTrack = audio
			s.tracks = append(s.tracks, audio)
		case codec.CodecID_PCMU:
			audio := &description.Media{
				Type: description.MediaTypeAudio,
				Formats: []format.Format{&format.Generic{
					PayloadTyp: v.PayloadType,
					ClockRat:   int(v.SampleRate),
					RTPMa:      fmt.Sprintf("PCMU/%d", v.SampleRate),
				}},
			}
			s.audioTrack = audio
			s.tracks = append(s.tracks, audio)
		}
		s.AddTrack(v)
	case ISubscriber:
		s.session = &description.Session{
			Medias: s.tracks,
		}
		if s.server != nil {
			s.stream = gortsplib.NewServerStream(s.server, s.session)
		}
	case VideoRTP:
		s.stream.WritePacketRTP(s.videoTrack, v.Packet)
	case AudioRTP:
		s.stream.WritePacketRTP(s.audioTrack, v.Packet)
	default:
		s.Subscriber.OnEvent(event)
	}
}
