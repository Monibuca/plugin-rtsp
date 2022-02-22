package rtsp

import (
	"errors"
	"time"
	"unsafe"

	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"github.com/Monibuca/utils/v3/codec"
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/aac"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
)

type RTSPClient struct {
	RTSPublisher
	Transport         gortsplib.Transport
	*gortsplib.Client `json:"-"`
}

// PullStream 从外部拉流
func (rtsp *RTSPClient) PullStream(streamPath string, rtspUrl string) (err error) {
	rtsp.Stream = &Stream{
		StreamPath: streamPath,
		Type:       "RTSP Pull",
		ExtraProp:  rtsp,
	}
	if result := rtsp.Publish(); result {
		rtsp.URL = rtspUrl
		if config.Reconnect {
			go func() {
				for rtsp.pullStream(); rtsp.Err() == nil; rtsp.pullStream() {
					Printf("reconnecting:%s in 5 seconds", rtspUrl)
					if rtsp.Transport == gortsplib.TransportTCP {
						rtsp.Transport = gortsplib.TransportUDP
					} else {
						rtsp.Transport = gortsplib.TransportTCP
					}
					time.Sleep(time.Second * 5)
				}
				if rtsp.IsTimeout {
					rtsp.processFunc = nil
					go rtsp.PullStream(streamPath, rtspUrl)
				}
			}()
		} else {
			go rtsp.pullStream()
		}
		return
	}
	return errors.New("publish badname")
}
func (rtsp *RTSPClient) PushStream(streamPath string, rtspUrl string) (err error) {
	if s := FindStream(streamPath); s != nil {
		var tracks gortsplib.Tracks
		var sub RTSPSubscriber
		sub.Type = "RTSP push out"
		sub.vt = s.WaitVideoTrack("h264", "h265")
		sub.at = s.WaitAudioTrack("aac", "pcma", "pcmu")
		ssrc := uintptr(unsafe.Pointer(&sub))
		var trackIds = 0
		if sub.vt != nil {
			trackId := trackIds
			var vtrack *gortsplib.Track
			var vpacketer rtp.Packetizer
			switch sub.vt.CodecID {
			case codec.CodecID_H264:
				if vtrack, err = gortsplib.NewTrackH264(96, &gortsplib.TrackConfigH264{
					SPS: sub.vt.ExtraData.NALUs[0],
					PPS: sub.vt.ExtraData.NALUs[1],
				}); err == nil {
					vpacketer = rtp.NewPacketizer(1200, 96, uint32(ssrc), &codecs.H264Payloader{}, rtp.NewFixedSequencer(1), 90000)
				} else {
					return err
				}
			case codec.CodecID_H265:
				vtrack = NewH265Track(96, sub.vt.ExtraData.NALUs)
				vpacketer = rtp.NewPacketizer(1200, 96, uint32(ssrc), &H265Payloader{}, rtp.NewFixedSequencer(1), 90000)
			}
			var st uint32
			onVideo := func(ts uint32, pack *VideoPack) {
				for _, nalu := range pack.NALUs {
					for _, pack := range vpacketer.Packetize(nalu, (ts-st)*90) {
						rtp, _ := pack.Marshal()
						rtsp.WritePacketRTP(trackId, rtp)
					}
				}
				st = ts
			}
			sub.OnVideo = func(ts uint32, pack *VideoPack) {
				if st = ts; st != 0 {
					sub.OnVideo = onVideo
				}
				onVideo(ts, pack)
			}
			tracks = append(tracks, vtrack)
			trackIds++
		}
		if sub.at != nil {
			var st uint32
			trackId := trackIds
			switch sub.at.CodecID {
			case codec.CodecID_PCMA, codec.CodecID_PCMU:
				atrack := NewG711Track(97, map[byte]string{7: "pcma", 8: "pcmu"}[sub.at.CodecID])
				apacketizer := rtp.NewPacketizer(1200, 97, uint32(ssrc), &codecs.G711Payloader{}, rtp.NewFixedSequencer(1), 8000)
				sub.OnAudio = func(ts uint32, pack *AudioPack) {
					for _, pack := range apacketizer.Packetize(pack.Raw, (ts-st)*8) {
						buf, _ := pack.Marshal()
						rtsp.WritePacketRTP(trackId, buf)
					}
					st = ts
				}
				tracks = append(tracks, atrack)
			case codec.CodecID_AAC:
				var mpegConf aac.MPEG4AudioConfig
				mpegConf.Decode(sub.at.ExtraData[2:])
				conf := &gortsplib.TrackConfigAAC{
					Type:              int(mpegConf.Type),
					SampleRate:        mpegConf.SampleRate,
					ChannelCount:      mpegConf.ChannelCount,
					AOTSpecificConfig: mpegConf.AOTSpecificConfig,
				}
				if atrack, err := gortsplib.NewTrackAAC(97, conf); err == nil {
					apacketizer := rtp.NewPacketizer(1200, 97, uint32(ssrc), &AACPayloader{}, rtp.NewFixedSequencer(1), uint32(mpegConf.SampleRate))
					sub.OnAudio = func(ts uint32, pack *AudioPack) {
						for _, pack := range apacketizer.Packetize(pack.Raw, (ts-st)*uint32(mpegConf.SampleRate)/1000) {
							buf, _ := pack.Marshal()
							rtsp.WritePacketRTP(trackId, buf)
						}
						st = ts
					}
					tracks = append(tracks, atrack)
				}
			}
		}
		return rtsp.StartPublishing(rtspUrl, tracks)
	}
	return errors.New("stream not exist")
}
func (client *RTSPClient) pullStream() {
	if client.Err() != nil {
		return
	}
	client.Client = &gortsplib.Client{
		OnPacketRTP: func(trackID int, payload []byte) {
			// Println("OnPacketRTP", trackID, len(payload))
			if f := client.processFunc[trackID]; f != nil {
				var clone []byte
				f(append(clone, payload...))
			}
		},
		ReadBufferSize: config.ReadBufferSize,
		Transport:      &client.Transport,
	}
	// parse URL
	u, err := base.ParseURL(client.URL)
	if err != nil {
		Printf("ParseURL:%s error:%v", client.URL, err)
		return
	}
	// connect to the server
	if err = client.Start(u.Scheme, u.Host); err != nil {
		Printf("connect:%s error:%v", client.URL, err)
		return
	}
	client.OnClose = func() {
		client.Client.Close()
	}
	//client.close should be after connected!
	defer client.Client.Close()
	var res *base.Response
	if res, err = client.Options(u); err != nil {
		Printf("option:%s error:%v", client.URL, err)
		return
	}
	Println(res)
	// find published tracks
	tracks, baseURL, res, err := client.Describe(u)
	if err != nil {
		Printf("Describe:%s error:%v", client.URL, err)
		return
	}
	Println(res)
	if client.processFunc == nil {
		client.setTracks(tracks)
	}
	for _, track := range tracks {
		if res, err = client.Setup(true, track, baseURL, 0, 0); err != nil {
			Printf("Setup:%s error:%v", baseURL.String(), err)
			return
		}
		Println(res)
	}
	// start reading tracks
	if res, err = client.Play(nil); err != nil {
		Printf("Play:%s error:%v", baseURL.String(), err)
		return
	}
	Println(res)
	// wait until a fatal error
	var fatalChan = make(chan error)
	go func() {
		fatalChan <- client.Wait()
	}()
	select {
	case err := <-fatalChan:
		Printf("Wait:%s error:%v", baseURL.String(), err)
	case <-client.Done():
		Printf("client:%s done", client.URL)
	}
}
