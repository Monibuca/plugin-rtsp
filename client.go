package rtsp

import (
	"errors"
	"time"

	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"github.com/aler9/gortsplib"
)

type RTSPClient struct {
	RTSPublisher
	Conn *gortsplib.ClientConn
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
				for rtsp.startStream(); rtsp.Err() == nil; rtsp.startStream() {
					Printf("reconnecting:%s in 5 seconds", rtspUrl)
					time.Sleep(time.Second * 5)
				}
				rtsp.Conn.Close()
				if rtsp.IsTimeout {
					go rtsp.PullStream(streamPath, rtspUrl)
				}
			}()
		} else {
			go func() {
				rtsp.startStream()
				rtsp.Conn.Close()
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
	// startTime := time.Now()
	//loggerTime := time.Now().Add(-10 * time.Second)
	conn, err := gortsplib.DialRead(client.URL)
	if err != nil {
		Printf("connect:%s error:%v", client.URL, err)
		return
	}
	client.Conn = conn
	tracks := conn.Tracks()
	client.setTracks(tracks)
	err = conn.ReadFrames(func(trackID int, streamType gortsplib.StreamType, payload []byte) {
		if streamType == gortsplib.StreamTypeRTP {
			client.processFunc[trackID](payload)
		}
	})
}
