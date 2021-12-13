package rtsp

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"

	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"github.com/aler9/gortsplib"
)

type RTSPublisher struct {
	*Stream     `json:"-"`
	URL         string
	stream      *gortsplib.ServerStream
	processFunc []func([]byte)
}

func (p *RTSPublisher) GetInfo() (info *RTSPStreamInfo) {
	info = &RTSPStreamInfo{
		URL:             p.URL,
		StreamPath:      p.StreamPath,
		Type:            p.Type,
		StartTime:       p.StartTime,
		SubscriberCount: len(p.Subscribers),
	}
	return
}
func (p *RTSPublisher) setTracks(tracks gortsplib.Tracks) {
	if p.processFunc != nil {
		p.processFunc = p.processFunc[:len(tracks)]
		return
	} else {
		p.processFunc = make([]func([]byte), len(tracks))
	}
	for i, track := range tracks {
		v, ok := track.Media.Attribute("rtpmap")
		if !ok {
			continue
		}

		fmtp := make(map[string]string)
		if v, ok := track.Media.Attribute("fmtp"); ok {
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

		v = strings.TrimSpace(v)
		vals := strings.Split(v, " ")
		if len(vals) != 2 {
			continue
		}
		timeScale := 0
		keyval := strings.Split(vals[1], "/")
		if i, err := strconv.Atoi(keyval[1]); err == nil {
			timeScale = i
		}
		if len(keyval) >= 2 {
			Printf("track %d is %s", i, keyval[0])
			switch strings.ToLower(keyval[0]) {
			case "h264":
				vt := p.NewRTPVideo(7)
				if conf, err := track.ExtractConfigH264(); err == nil {
					vt.PushNalu(0, 0, conf.SPS, conf.PPS)
				}
				p.processFunc[i] = vt.Push
			case "h265", "hevc":
				vt := p.NewRTPVideo(12)
				if v, ok := fmtp["sprop-vps"]; ok {
					vps, _ := base64.StdEncoding.DecodeString(v)
					vt.PushNalu(0, 0, vps)
				}
				if v, ok := fmtp["sprop-sps"]; ok {
					sps, _ := base64.StdEncoding.DecodeString(v)
					vt.PushNalu(0, 0, sps)
				}
				if v, ok := fmtp["sprop-pps"]; ok {
					pps, _ := base64.StdEncoding.DecodeString(v)
					vt.PushNalu(0, 0, pps)
				}
				p.processFunc[i] = vt.Push
			case "pcma":
				at := p.NewRTPAudio(7)
				at.SoundRate = timeScale
				at.SoundSize = 16
				if len(keyval) >= 3 {
					x, _ := strconv.Atoi(keyval[2])
					at.Channels = byte(x)
				} else {
					at.Channels = 1
				}
				at.ExtraData = []byte{(at.CodecID << 4) | (1 << 1)}
				p.processFunc[i] = at.Push
			case "pcmu":
				at := p.NewRTPAudio(8)
				at.SoundRate = timeScale
				at.SoundSize = 16
				if len(keyval) >= 3 {
					x, _ := strconv.Atoi(keyval[2])
					at.Channels = byte(x)
				} else {
					at.Channels = 1
				}
				at.ExtraData = []byte{(at.CodecID << 4) | (1 << 1)}
				p.processFunc[i] = at.Push
			case "mpeg4-generic":
				at := p.NewRTPAudio(0)
				if config, ok := fmtp["config"]; ok {
					asc, _ := hex.DecodeString(config)
					at.SetASC(asc)
				} else {
					Println("aac no config")
				}
				at.SoundSize = 16
				p.processFunc[i] = at.Push
			}
		}
	}
}
