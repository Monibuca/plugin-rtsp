package rtsp

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aler9/gortsplib"
	"go.uber.org/zap"
	. "m7s.live/engine/v4"
	"m7s.live/engine/v4/common"
	. "m7s.live/engine/v4/track"
)

type RTSPPublisher struct {
	Publisher
	Tracks []common.AVTrack `json:"-"`
	RTSPIO
}

func (p *RTSPPublisher) SetTracks() error {
	p.Tracks = make([]common.AVTrack, len(p.tracks))
	defer func() {
		for i, track := range p.Tracks {
			if track == nil {
				p.Info("unknown track", zap.String("codec", p.Tracks[i].String()))
				continue
			}
			p.Info("set track", zap.Int("trackId", i), zap.String("name", track.GetBase().Name))
		}
	}()
	for trackId, track := range p.tracks {
		md := track.MediaDescription()
		v, ok := md.Attribute("rtpmap")
		if !ok {
			return errors.New("rtpmap attribute not found")
		}
		v = strings.TrimSpace(v)
		vals := strings.Split(v, " ")
		if len(vals) != 2 {
			continue
		}
		fmtp := make(map[string]string)
		if v, ok = md.Attribute("fmtp"); ok {
			if tmp := strings.SplitN(v, " ", 2); len(tmp) == 2 {
				for _, kv := range strings.Split(tmp[1], ";") {
					kv = strings.Trim(kv, " ")

					if len(kv) == 0 {
						continue
					}
					tmp := strings.SplitN(kv, "=", 2)
					if len(tmp) == 2 {
						fmtp[strings.TrimSpace(tmp[0])] = tmp[1]
					}
				}
			}
		}
		timeScale := 0
		keyval := strings.Split(vals[1], "/")
		if i, err := strconv.Atoi(keyval[1]); err == nil {
			timeScale = i
		}
		if len(keyval) >= 2 {
			switch strings.ToLower(keyval[0]) {
			case "h264":
				vt := NewH264(p.Stream)
				if payloadType, err := strconv.Atoi(vals[0]); err == nil {
					vt.PayloadType = byte(payloadType)
				}
				p.Tracks[trackId] = vt
				t := track.(*gortsplib.TrackH264)
				if len(t.SPS) > 0 {
					vt.WriteSliceBytes(t.SPS)
				}
				if len(t.PPS) > 0 {
					vt.WriteSliceBytes(t.PPS)
				}
			case "h265", "hevc":
				vt := NewH265(p.Stream)
				if payloadType, err := strconv.Atoi(vals[0]); err == nil {
					vt.PayloadType = byte(payloadType)
				}
				p.Tracks[trackId] = vt
				if v, ok := fmtp["sprop-vps"]; ok {
					vps, _ := base64.StdEncoding.DecodeString(v)
					vt.WriteSliceBytes(vps)
				}
				if v, ok := fmtp["sprop-sps"]; ok {
					sps, _ := base64.StdEncoding.DecodeString(v)
					vt.WriteSliceBytes(sps)
				}
				if v, ok := fmtp["sprop-pps"]; ok {
					pps, _ := base64.StdEncoding.DecodeString(v)
					vt.WriteSliceBytes(pps)
				}
			case "pcma":
				at := NewG711(p.Stream, true)
				if payloadType, err := strconv.Atoi(vals[0]); err == nil {
					at.PayloadType = byte(payloadType)
				}
				p.Tracks[trackId] = at
				at.SampleRate = uint32(timeScale)
				if len(keyval) >= 3 {
					x, _ := strconv.Atoi(keyval[2])
					at.Channels = byte(x)
				} else {
					at.Channels = 1
				}
				at.AVCCHead = []byte{(byte(at.CodecID) << 4) | (1 << 1)}
			case "pcmu":
				at := NewG711(p.Stream, false)
				if payloadType, err := strconv.Atoi(vals[0]); err == nil {
					at.PayloadType = byte(payloadType)
				}
				p.Tracks[trackId] = at
				at.SampleRate = uint32(timeScale)
				if len(keyval) >= 3 {
					x, _ := strconv.Atoi(keyval[2])
					at.Channels = byte(x)
				} else {
					at.Channels = 1
				}
				at.AVCCHead = []byte{(byte(at.CodecID) << 4) | (1 << 1)}
			case "mpeg4-generic":
				at := NewAAC(p.Stream)
				if payloadType, err := strconv.Atoi(vals[0]); err == nil {
					at.PayloadType = byte(payloadType)
				}
				p.Tracks[trackId] = at
				if config, ok := fmtp["config"]; ok {
					asc, _ := hex.DecodeString(config)
					// 复用AVCC写入逻辑，解析出AAC的配置信息
					at.WriteSequenceHead(append([]byte{0xAF, 0x00}, asc...))
				} else {
					RTSPPlugin.Warn("aac no config")
				}
			default:
				return fmt.Errorf("unsupport codec:%s", keyval[0])
			}
		}
	}
	return nil
}
