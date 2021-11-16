package rtsp

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/aler9/gortsplib"
	psdp "github.com/pion/sdp/v3"
)

// func NewTrackAAC(payloadType uint8, conf *gortsplib.TrackConfigAAC) (*gortsplib.Track, error) {
// 	mpegConf, err := aac.MPEG4AudioConfig{
// 		Type:              aac.MPEG4AudioType(conf.Type),
// 		SampleRate:        conf.SampleRate,
// 		ChannelCount:      conf.ChannelCount,
// 		AOTSpecificConfig: conf.AOTSpecificConfig,
// 	}.Encode()
// 	if err != nil {
// 		return nil, err
// 	}

// 	typ := strconv.FormatInt(int64(payloadType), 10)

// 	return &gortsplib.Track{
// 		Media: &psdp.MediaDescription{
// 			MediaName: psdp.MediaName{
// 				Media:   "audio",
// 				Protos:  []string{"RTP", "AVP"},
// 				Formats: []string{typ},
// 			},
// 			Attributes: []psdp.Attribute{
// 				{
// 					Key: "rtpmap",
// 					Value: typ + " mpeg4-generic/" + strconv.FormatInt(int64(conf.SampleRate), 10) +
// 						"/" + strconv.FormatInt(int64(conf.ChannelCount), 10),
// 				},
// 				{
// 					Key: "fmtp",
// 					Value: typ + " profile-level-id=1; " +
// 						"mode=AAC-hbr; " +
// 						"sizelength=6; " +
// 						"indexlength=2; " +
// 						"indexdeltalength=2; " +
// 						"config=" + hex.EncodeToString(mpegConf),
// 				},
// 			},
// 		},
// 	}, nil
// }
func NewG711Track(payloadType uint8, law string) *gortsplib.Track {
	return &gortsplib.Track{
		Media: &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   "audio",
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{strconv.FormatInt(int64(payloadType), 10)}},
			Attributes: []psdp.Attribute{
				{
					Key:   "rtpmap",
					Value: fmt.Sprintf("%d %s/8000/1", payloadType, law),
				},
			},
		},
	}
}
func NewH265Track(payloadType uint8, sprop [][]byte) *gortsplib.Track {
	return &gortsplib.Track{
		Media: &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   "video",
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{fmt.Sprintf("%d", payloadType)},
			},
			Attributes: []psdp.Attribute{
				{
					Key:   "rtpmap",
					Value: fmt.Sprintf("%d H265/90000", payloadType),
				},
				{
					Key:   "fmtp",
					Value: fmt.Sprintf("%d packetization-mode=1;sprop-vps=%s;sprop-sps=%s;sprop-pps=%s;", payloadType, base64.StdEncoding.EncodeToString(sprop[0]), base64.StdEncoding.EncodeToString(sprop[1]), base64.StdEncoding.EncodeToString(sprop[2])),
				},
			},
		},
	}
}
