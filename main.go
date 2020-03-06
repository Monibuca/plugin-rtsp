package rtspplugin

import (
	"bytes"
	"log"
	"strings"

	. "github.com/Monibuca/engine"
	. "github.com/Monibuca/engine/avformat"
	"github.com/Monibuca/engine/util"
)

var config = struct {
	BufferLength int
	AutoPublish  bool
	RtspURL      string
}{2048, true, "rtsp://localhost/${streamPath}"}

func init() {
	InstallPlugin(&PluginConfig{
		Name:    "RTSP",
		Version: "1.0.0",
		Config:  &config,
		Run: func() {
			if config.AutoPublish {
				OnSubscribeHooks.AddHook(func(s *OutputStream) {
					if s.Publisher == nil {
						new(RTSP).Publish(s.StreamPath, strings.Replace(config.RtspURL, "${streamPath}", "s.StreamPath"))
					}
				})
			}
		},
	})
}

type RTSP struct {
	InputStream
	*RtspClient
	RTSPInfo
}
type RTSPInfo struct {
	SyncCount int64
	RoomInfo  *RoomInfo
}

func (rtsp *RTSP) run() {

	fuBuffer := []byte{}
	iframeHead := []byte{0x17, 0x01, 0, 0, 0}
	pframeHead := []byte{0x27, 0x01, 0, 0, 0}
	spsHead := []byte{0xE1, 0, 0}
	ppsHead := []byte{0x01, 0, 0}
	nalLength := []byte{0, 0, 0, 0}
	r := bytes.NewBuffer([]byte{})
	av := NewAVPacket(FLV_TAG_TYPE_VIDEO)
	handleNALU := func(nalType byte, payload []byte, ts int64) {
		rtsp.SyncCount++
		vl := len(payload)
		switch nalType {
		case NALU_SPS:
			r.Write(RTMP_AVC_HEAD)
			util.BigEndian.PutUint16(spsHead[1:], uint16(vl))
			r.Write(spsHead)
			r.Write(payload)
		case NALU_PPS:
			util.BigEndian.PutUint16(ppsHead[1:], uint16(vl))
			r.Write(ppsHead)
			r.Write(payload)
			av.VideoFrameType = 1
			av.Payload = r.Bytes()
			rtsp.PushVideo(av)
		case NALU_IDR_Picture:
			av = NewAVPacket(FLV_TAG_TYPE_VIDEO)
			r.Reset()
			av.VideoFrameType = 1
			av.Timestamp = uint32(ts)
			util.BigEndian.PutUint24(iframeHead[2:], 0)
			r.Write(iframeHead)
			util.BigEndian.PutUint32(nalLength, uint32(vl))
			r.Write(nalLength)
			rtsp.PushVideo(av)
		case NALU_Non_IDR_Picture:
			av = NewAVPacket(FLV_TAG_TYPE_VIDEO)
			r.Reset()
			av.VideoFrameType = 2
			av.Timestamp = uint32(ts)
			util.BigEndian.PutUint24(pframeHead[2:], 0)
			r.Write(iframeHead)
			util.BigEndian.PutUint32(nalLength, uint32(vl))
			r.Write(nalLength)
			rtsp.PushVideo(av)
		}
	}
	for {
		select {
		case <-rtsp.Done():
			return
		case data, ok := <-rtsp.OutGoing:
			if ok && data[0] == 36 {
				if data[1] == 0 {
					cc := data[4] & 0xF
					//rtp header
					rtphdr := 12 + cc*4

					//packet time
					ts := (int64(data[8]) << 24) + (int64(data[9]) << 16) + (int64(data[10]) << 8) + (int64(data[11]))

					//packet number
					packno := (int64(data[6]) << 8) + int64(data[7])
					if false {
						log.Println("packet num", packno)
					}

					nalType := data[4+rtphdr] & 0x1F

					if nalType >= 1 && nalType <= 23 {
						handleNALU(nalType, data[4+rtphdr:], ts)
					} else if nalType == 28 {
						isStart := data[4+rtphdr+1]&0x80 != 0
						isEnd := data[4+rtphdr+1]&0x40 != 0
						nalType := data[4+rtphdr+1] & 0x1F
						//nri := (data[4+rtphdr+1]&0x60)>>5
						nal := data[4+rtphdr]&0xE0 | data[4+rtphdr+1]&0x1F
						if isStart {
							fuBuffer = []byte{0}
						}
						fuBuffer = append(fuBuffer, data[4+rtphdr+2:]...)
						if isEnd {
							fuBuffer[0] = nal
							handleNALU(nalType, fuBuffer, ts)
						}
					}

				} else if data[1] == 2 {
					// audio
					cc := data[4] & 0xF
					rtphdr := 12 + cc*4
					//or not payload := data[4+rtphdr:]
					payload := data[4+rtphdr+4:]
					av := NewAVPacket(FLV_TAG_TYPE_AUDIO)
					av.Payload = payload
					rtsp.PushAudio(av)
				}
			}
		}
	}
}
func (rtsp *RTSP) Publish(streamPath string, rtspUrl string) (result bool) {
	if result = rtsp.InputStream.Publish(streamPath, rtsp); result {
		rtsp.RTSPInfo.RoomInfo = &rtsp.Room.RoomInfo
		rtsp.RtspClient = RtspClientNew()
		if status, message := rtsp.RtspClient.Client(rtspUrl); !status {
			log.Println(message)
			return false
		}
		go rtsp.run()
	}
	return
}
