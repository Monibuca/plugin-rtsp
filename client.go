package rtsp

import (
	"errors"
	"time"

	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

type RTSPClient struct {
	RTSPublisher
	gortsplib.Client `json:"-"`
}

// PullStream 从外部拉流
func (rtsp *RTSPClient) PullStream(streamPath string, rtspUrl string) (err error) {
	rtsp.Stream = &Stream{
		StreamPath: streamPath,
		Type:       "RTSP Pull",
		ExtraProp:  rtsp,
	}
	rtsp.OnPacketRTP = func(trackID int, payload []byte) {
		rtsp.processFunc[trackID](payload)
	}
	if result := rtsp.Publish(); result {
		rtsp.URL = rtspUrl
		if config.Reconnect {
			go func() {
				for rtsp.startStream(); rtsp.Err() == nil; rtsp.startStream() {
					Printf("reconnecting:%s in 5 seconds", rtspUrl)
					time.Sleep(time.Second * 5)
				}
				rtsp.Client.Close()
				if rtsp.IsTimeout {
					go rtsp.PullStream(streamPath, rtspUrl)
				}
			}()
		} else {
			go func() {
				rtsp.startStream()
				rtsp.Client.Close()
			}()
		}
		return
	}
	return errors.New("publish badname")
}

func (client *RTSPClient) startStream() {
	if client.Err() != nil {
		return
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
	var res *base.Response
	if res, err = client.Options(u); err != nil {
		Printf("option:%s error:%v", client.URL, err)
		return
	}
	Println(res)
	// find published tracks
	tracks, baseURL, res, err := client.Describe(u)
	if err != nil {
		Printf("Describe:%s error:%v", baseURL.String(), err)
		return
	}
	Println(res)
	client.setTracks(tracks)
	for _, track := range tracks {
		if res, err = client.Setup(true, baseURL, track, 0, 0); err != nil {
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
	err = client.Wait()
	Printf("Wait:%s error:%v", baseURL.String(), err)
}
