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
			extra := v.GetDecoderConfiguration().Raw
			if vtrack, err := gortsplib.NewTrackH264(96, extra[0], extra[1], nil); err == nil {
				s.videoTrackId = len(s.tracks)
				s.tracks = append(s.tracks, vtrack)
			}
		case codec.CodecID_H265:
			if vtrack, err := NewH265Track(96, v.GetDecoderConfiguration().Raw); err == nil {
				s.videoTrackId = len(s.tracks)
				s.tracks = append(s.tracks, vtrack)
			}
		}
		s.AddTrack(v)
	case *track.Audio:
		switch v.CodecID {
		case codec.CodecID_AAC:
			var mpegConf aac.MPEG4AudioConfig
			mpegConf.Decode(v.GetDecoderConfiguration().Raw)
			if atrack, err := gortsplib.NewTrackAAC(97, int(mpegConf.Type), mpegConf.SampleRate, mpegConf.ChannelCount, mpegConf.AOTSpecificConfig); err == nil {
				s.audioTrackId = len(s.tracks)
				s.tracks = append(s.tracks, atrack)
			}
		case codec.CodecID_PCMA:
			s.audioTrackId = len(s.tracks)
			s.tracks = append(s.tracks, NewPCMATrack())
		case codec.CodecID_PCMU:
			s.audioTrackId = len(s.tracks)
			s.tracks = append(s.tracks, gortsplib.NewTrackPCMU())
		}
		s.AddTrack(v)
	case ISubscriber:
		s.stream = gortsplib.NewServerStream(s.tracks)
	case *AudioFrame:
		for _, pack := range v.RTP {
			s.stream.WritePacketRTP(s.audioTrackId, &pack.Packet)
		}
	case *VideoFrame:
		for _, pack := range v.RTP {
			s.stream.WritePacketRTP(s.videoTrackId, &pack.Packet)
		}
	default:
		s.Subscriber.OnEvent(event)
	}
}
