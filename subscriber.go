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
			s.videoTrack = &description.Media{
				Type: description.MediaTypeVideo,
				Formats: []format.Format{&format.H264{
					PacketizationMode: 1,
					PayloadTyp:        v.PayloadType,
					SPS:               v.ParamaterSets[0],
					PPS:               v.ParamaterSets[1],
				}},
			}
		case codec.CodecID_H265:
			s.videoTrack = &description.Media{
				Type: description.MediaTypeVideo,
				Formats: []format.Format{&format.H265{
					PayloadTyp: v.PayloadType,
					VPS:        v.ParamaterSets[0],
					SPS:        v.ParamaterSets[1],
					PPS:        v.ParamaterSets[2],
				}},
			}
		case codec.CodecID_AV1:
			var idx, profile, tail int
			idx = int(v.ParamaterSets[1][0])
			profile = int(v.ParamaterSets[1][1])
			tail = int(v.ParamaterSets[1][2])
			s.videoTrack = &description.Media{
				Type: description.MediaTypeVideo,
				Formats: []format.Format{&format.AV1{
					PayloadTyp: v.PayloadType,
					LevelIdx:   &idx,
					Profile:    &profile,
					Tier:       &tail,
				}},
			}
		}
		if s.videoTrack != nil {
			s.tracks = append(s.tracks, s.videoTrack)
			s.AddTrack(v)
		}
	case *track.Audio:
		if s.Audio != nil {
			return
		}
		switch v.CodecID {
		case codec.CodecID_AAC:
			s.audioTrack = &description.Media{
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
		case codec.CodecID_PCMA:
			s.audioTrack = &description.Media{
				Type: description.MediaTypeAudio,
				Formats: []format.Format{&format.Generic{
					PayloadTyp: v.PayloadType,
					ClockRat:   int(v.SampleRate),
					RTPMa:      fmt.Sprintf("PCMA/%d", v.SampleRate),
				}},
			}
		case codec.CodecID_PCMU:
			s.audioTrack = &description.Media{
				Type: description.MediaTypeAudio,
				Formats: []format.Format{&format.Generic{
					PayloadTyp: v.PayloadType,
					ClockRat:   int(v.SampleRate),
					RTPMa:      fmt.Sprintf("PCMU/%d", v.SampleRate),
				}},
			}
		case codec.CodecID_OPUS:
			s.audioTrack = &description.Media{
				Type: description.MediaTypeAudio,
				Formats: []format.Format{&format.Opus{
					PayloadTyp: v.PayloadType,
				}},
			}
		}
		if s.audioTrack != nil {
			s.tracks = append(s.tracks, s.audioTrack)
			s.AddTrack(v)
		}
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
