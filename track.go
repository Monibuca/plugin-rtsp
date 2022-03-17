package rtsp

import (
	"encoding/base64"
	"fmt"

	"github.com/aler9/gortsplib"
	psdp "github.com/pion/sdp/v3"
)

type TrackPCMA struct {
	*gortsplib.TrackPCMU
}

func NewPCMATrack() *TrackPCMA {
	return &TrackPCMA{
		gortsplib.NewTrackPCMU(),
	}
}
func (t *TrackPCMA) MediaDescription() *psdp.MediaDescription {
	md := t.TrackPCMU.MediaDescription()
	md.Attributes[0].Value = "0 PCMA/8000"
	return md
}

func NewH265Track(payloadType uint8, sprop [][]byte) (gortsplib.Track, error) {
	return gortsplib.NewTrackGeneric("video", []string{fmt.Sprintf("%d", payloadType)}, fmt.Sprintf("%d H265/90000", payloadType), fmt.Sprintf("%d packetization-mode=1;sprop-vps=%s;sprop-sps=%s;sprop-pps=%s;", payloadType, base64.StdEncoding.EncodeToString(sprop[0]), base64.StdEncoding.EncodeToString(sprop[1]), base64.StdEncoding.EncodeToString(sprop[2])))
}
