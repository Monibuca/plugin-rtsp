package rtsp

import (
	"encoding/base64"
	"fmt"

	"github.com/aler9/gortsplib"
	psdp "github.com/pion/sdp/v3"
)

const (
	pcma = "PCMA/8000"
	pcmu = "PCMU/8000"
)

type TrackG711 struct {
	*gortsplib.TrackPCMU
	payloadType byte
	format      string
}

func NewG711(payloadType byte, isPcma bool) *TrackG711 {
	format := pcma
	if !isPcma {
		format = pcmu
	}
	format = fmt.Sprintf("%d %s", payloadType, format)
	return &TrackG711{
		gortsplib.NewTrackPCMU(),
		payloadType,
		format,
	}
}
func (t *TrackG711) MediaDescription() *psdp.MediaDescription {
	md := t.TrackPCMU.MediaDescription()
	md.MediaName.Formats[0] = fmt.Sprintf("%d", t.payloadType)
	md.Attributes[0].Value = t.format
	return md
}

func NewH265Track(payloadType uint8, sprop [][]byte) (gortsplib.Track, error) {
	return gortsplib.NewTrackGeneric("video", []string{fmt.Sprintf("%d", payloadType)}, fmt.Sprintf("%d H265/90000", payloadType), fmt.Sprintf("%d packetization-mode=1; sprop-vps=%s; sprop-sps=%s; sprop-pps=%s;", payloadType, base64.StdEncoding.EncodeToString(sprop[0]), base64.StdEncoding.EncodeToString(sprop[1]), base64.StdEncoding.EncodeToString(sprop[2])))
}
