package rtsp

import (
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/aac"
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
		switch v.CodecID {
		case codec.CodecID_H264:
			extra := v.DecoderConfiguration.Raw
			vtrack := &gortsplib.TrackH264{
				PayloadType: v.DecoderConfiguration.PayloadType, SPS: extra[0], PPS: extra[1],
			}
			s.videoTrackId = len(s.tracks)
			s.tracks = append(s.tracks, vtrack)
		case codec.CodecID_H265:
			vtrack := &gortsplib.TrackH265{
				PayloadType: v.DecoderConfiguration.PayloadType, VPS: v.DecoderConfiguration.Raw[0], SPS: v.DecoderConfiguration.Raw[1], PPS: v.DecoderConfiguration.Raw[2],
			}
			s.videoTrackId = len(s.tracks)
			s.tracks = append(s.tracks, vtrack)
		}
		s.AddTrack(v)
	case *track.Audio:
		switch v.CodecID {
		case codec.CodecID_AAC:
			var mpegConf aac.MPEG4AudioConfig
			mpegConf.Unmarshal(v.DecoderConfiguration.Raw)
			atrack := &gortsplib.TrackAAC{
				PayloadType: v.DecoderConfiguration.PayloadType, Config: &mpegConf, SizeLength: 13, IndexLength: 3, IndexDeltaLength: 3,
			}
			s.audioTrackId = len(s.tracks)
			s.tracks = append(s.tracks, atrack)
		case codec.CodecID_PCMA:
			s.audioTrackId = len(s.tracks)

			s.tracks = append(s.tracks, &gortsplib.TrackPCMA{})
		case codec.CodecID_PCMU:
			s.audioTrackId = len(s.tracks)
			s.tracks = append(s.tracks, &gortsplib.TrackPCMU{})
		}
		s.AddTrack(v)
	case ISubscriber:
		s.stream = gortsplib.NewServerStream(s.tracks)
	case VideoRTP:
		s.stream.WritePacketRTP(s.videoTrackId, &v.Packet, s.Video.Frame.PTS == s.Video.Frame.DTS)
	case AudioRTP:
		s.stream.WritePacketRTP(s.audioTrackId, &v.Packet, s.Audio.Frame.PTS == s.Audio.Frame.DTS)
	default:
		s.Subscriber.OnEvent(event)
	}
}
