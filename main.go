package rtspplugin

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	. "github.com/Monibuca/engine"
	. "github.com/Monibuca/engine/avformat"
	"github.com/Monibuca/engine/util"
)

var collection = sync.Map{}
var config = struct {
	BufferLength int
	AutoPull     bool
	RemoteAddr   string
}{2048, true, "rtsp://localhost/${streamPath}"}

func init() {
	InstallPlugin(&PluginConfig{
		Name:    "RTSP",
		Type:    PLUGIN_PUBLISHER | PLUGIN_HOOK,
		Version: "1.0.0",
		Config:  &config,
		UI:      util.CurrentDir("dashboard", "ui", "plugin-rtsp.min.js"),
		Run:     runPlugin,
	})
}
func runPlugin() {
	if config.AutoPull {
		OnSubscribeHooks.AddHook(func(s *OutputStream) {
			if s.Publisher == nil {
				new(RTSP).Publish(s.StreamPath, strings.Replace(config.RemoteAddr, "${streamPath}", s.StreamPath, -1))
			}
		})
	}
	http.HandleFunc("/rtsp/list", func(w http.ResponseWriter, r *http.Request) {
		sse := util.NewSSE(w, r.Context())
		var err error
		for tick := time.NewTicker(time.Second); err == nil; <-tick.C {
			var info []*RTSPInfo
			collection.Range(func(key, value interface{}) bool {
				rtsp := value.(*RTSP)
				pinfo := &rtsp.RTSPInfo
				pinfo.BufferRate = len(rtsp.OutGoing) * 100 / config.BufferLength
				info = append(info, pinfo)
				return true
			})
			err = sse.WriteJSON(info)
		}
	})
	http.HandleFunc("/rtsp/pull", func(w http.ResponseWriter, r *http.Request) {
		targetURL := r.URL.Query().Get("target")
		streamPath := r.URL.Query().Get("streamPath")
		var err error
		if err == nil {
			new(RTSP).Publish(streamPath, targetURL)
			w.Write([]byte(`{"code":0}`))
		} else {
			w.Write([]byte(fmt.Sprintf(`{"code":1,"msg":"%s"}`, err.Error())))
		}
	})
}

type RTSP struct {
	InputStream
	*RtspClient
	RTSPInfo
}
type RTSPInfo struct {
	SyncCount  int64
	Header     *string
	BufferRate int
	RoomInfo   *RoomInfo
}

func (rtsp *RTSP) run() {
	fuBuffer := []byte{}
	iframeHead := []byte{0x17, 0x01, 0, 0, 0}
	pframeHead := []byte{0x27, 0x01, 0, 0, 0}
	spsHead := []byte{0xE1, 0, 0}
	ppsHead := []byte{0x01, 0, 0}
	nalLength := []byte{0, 0, 0, 0}

	av := NewAVPacket(FLV_TAG_TYPE_VIDEO)
	avcsent := false
	aacsent := false
	handleNALU := func(nalType byte, payload []byte, ts int64) {
		rtsp.SyncCount++
		vl := len(payload)
		switch nalType {
		case NALU_SPS:
			r := bytes.NewBuffer([]byte{})
			r.Write(RTMP_AVC_HEAD)
			util.BigEndian.PutUint16(spsHead[1:], uint16(vl))
			r.Write(spsHead)
			r.Write(payload)
		case NALU_PPS:
			r := bytes.NewBuffer([]byte{})
			util.BigEndian.PutUint16(ppsHead[1:], uint16(vl))
			r.Write(ppsHead)
			r.Write(payload)
			av.VideoFrameType = 1
			av.Payload = r.Bytes()
			rtsp.PushVideo(av)
			avcsent = true
		case NALU_IDR_Picture:
			if !avcsent {
				r := bytes.NewBuffer([]byte{})
				av = NewAVPacket(FLV_TAG_TYPE_VIDEO)
				r.Write(RTMP_AVC_HEAD)
				util.BigEndian.PutUint16(spsHead[1:], uint16(len(rtsp.SPS)))
				r.Write(spsHead)
				r.Write(rtsp.SPS)
				util.BigEndian.PutUint16(ppsHead[1:], uint16(len(rtsp.PPS)))
				r.Write(ppsHead)
				r.Write(rtsp.PPS)
				av.VideoFrameType = 1
				av.Payload = r.Bytes()
				rtsp.PushVideo(av)
				avcsent = true
			}
			av = NewAVPacket(FLV_TAG_TYPE_VIDEO)
			r := bytes.NewBuffer([]byte{})
			av.VideoFrameType = 1
			av.Timestamp = uint32(ts)
			util.BigEndian.PutUint24(iframeHead[2:], 0)
			r.Write(iframeHead)
			util.BigEndian.PutUint32(nalLength, uint32(vl))
			r.Write(nalLength)
			r.Write(payload)
			av.Payload = r.Bytes()
			rtsp.PushVideo(av)
		case NALU_Non_IDR_Picture:
			av = NewAVPacket(FLV_TAG_TYPE_VIDEO)
			r := bytes.NewBuffer([]byte{})
			av.VideoFrameType = 2
			av.Timestamp = uint32(ts)
			util.BigEndian.PutUint24(pframeHead[2:], 0)
			r.Write(pframeHead)
			util.BigEndian.PutUint32(nalLength, uint32(vl))
			r.Write(nalLength)
			r.Write(payload)
			av.Payload = r.Bytes()
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
					//packno := (int64(data[6]) << 8) + int64(data[7])
					data = data[4+rtphdr:]
					nalType := data[0] & 0x1F

					if nalType >= 1 && nalType <= 23 {
						handleNALU(nalType, data, ts)
					} else if nalType == 28 {
						isStart := data[1]&0x80 != 0
						isEnd := data[1]&0x40 != 0
						nalType := data[1] & 0x1F
						//nri := (data[1]&0x60)>>5
						nal := data[0]&0xE0 | data[1]&0x1F
						if isStart {
							fuBuffer = []byte{0}
						}
						fuBuffer = append(fuBuffer, data[2:]...)
						if isEnd {
							fuBuffer[0] = nal
							handleNALU(nalType, fuBuffer, ts)
						}
					}

				} else if data[1] == 2 {
					// audio
					if !aacsent {
						av := NewAVPacket(FLV_TAG_TYPE_AUDIO)
						av.Payload = append([]byte{0xAF, 0x00}, rtsp.AudioSpecificConfig...)
						rtsp.PushAudio(av)
						aacsent = true
					}
					cc := data[4] & 0xF
					rtphdr := 12 + cc*4
					payload := data[4+rtphdr:]
					auHeaderLen := (int16(payload[0]) << 8) + int16(payload[1])
					auHeaderLen = auHeaderLen >> 3
					auHeaderCount := int(auHeaderLen / 2)
					var auLenArray []int
					for iIndex := 0; iIndex < int(auHeaderCount); iIndex++ {
						auHeaderInfo := (int16(payload[2+2*iIndex]) << 8) + int16(payload[2+2*iIndex+1])
						auLen := auHeaderInfo >> 3
						auLenArray = append(auLenArray, int(auLen))
					}
					startOffset := 2 + 2*auHeaderCount
					for _, auLen := range auLenArray {
						endOffset := startOffset + auLen
						av := NewAVPacket(FLV_TAG_TYPE_AUDIO)
						addHead := []byte{0xAF, 0x01}
						av.Payload = append(addHead, payload[startOffset:endOffset]...)
						rtsp.PushAudio(av)
						startOffset = startOffset + auLen
					}
				}
			}
		}
	}
}
func (rtsp *RTSP) Publish(streamPath string, rtspUrl string) (result bool) {
	if result = rtsp.InputStream.Publish(streamPath, rtsp); result {
		rtsp.RTSPInfo.RoomInfo = &rtsp.Room.RoomInfo
		rtsp.RtspClient = RtspClientNew(config.BufferLength)
		rtsp.RTSPInfo.Header = &rtsp.RtspClient.Header
		if status, message := rtsp.RtspClient.Client(rtspUrl); !status {
			log.Println(message)
			return false
		}
		collection.Store(streamPath, rtsp)
		go rtsp.run()
	}
	return
}
