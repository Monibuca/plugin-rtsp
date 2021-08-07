package rtsp

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/teris-io/shortid"
)

type RTPPack struct {
	Type RTPType
	rtp.Packet
}
type SessionType int

const (
	SESSION_TYPE_PUSHER SessionType = iota
	SESSEION_TYPE_PLAYER
)

func (st SessionType) String() string {
	switch st {
	case SESSION_TYPE_PUSHER:
		return "pusher"
	case SESSEION_TYPE_PLAYER:
		return "player"
	}
	return "unknow"
}

type RTPType int

const (
	RTP_TYPE_AUDIO RTPType = iota
	RTP_TYPE_VIDEO
	RTP_TYPE_AUDIOCONTROL
	RTP_TYPE_VIDEOCONTROL
)

type TransType int

const (
	TRANS_TYPE_TCP TransType = iota
	TRANS_TYPE_UDP
)

func (tt TransType) String() string {
	switch tt {
	case TRANS_TYPE_TCP:
		return "TCP"
	case TRANS_TYPE_UDP:
		return "UDP"
	}
	return "unknow"
}

const UDP_BUF_SIZE = 1048576

func (session *RTSP) SessionString() string {
	return fmt.Sprintf("session[%v][%v][%s][%s][%s]", session.Type, session.TransType, session.StreamPath, session.ID, session.Conn.RemoteAddr().String())
}

func (session *RTSP) Stop() {
	if session.Stream != nil {
		session.Close()
	}
	if session.Conn != nil {
		session.connRW.Flush()
		session.Conn.Close()
		session.Conn = nil
	}
	if session.UDPClient != nil {
		session.UDPClient.Stop()
		session.UDPClient = nil
	}
	if session.UDPServer != nil {
		session.UDPServer.Stop()
		session.UDPServer = nil
	}
}

// AcceptPush 接受推流
func (session *RTSP) AcceptPush() {
	defer session.Stop()
	buf2 := make([]byte, 2)
	timer := time.Unix(0, 0)
	for {
		buf1, err := session.connRW.ReadByte()
		if err != nil {
			Println(err)
			return
		}
		if buf1 == 0x24 { //rtp data
			if buf1, err = session.connRW.ReadByte(); err != nil {
				Println(err)
				return
			}
			if _, err := io.ReadFull(session.connRW, buf2); err != nil {
				Println(err)
				return
			}
			channel := int(buf1)
			rtpLen := int(binary.BigEndian.Uint16(buf2))
			rtpBytes := make([]byte, rtpLen)
			if _, err := io.ReadFull(session.connRW, rtpBytes); err != nil {
				Println(err)
				return
			}

			// t := pack.Timestamp / 90
			switch channel {
			case session.aRTPChannel:
				// pack.Type = RTP_TYPE_AUDIO
				if session.RtpAudio != nil {
					elapsed := time.Since(timer)
					if elapsed >= 30*time.Second {
						Println("Recv an audio RTP package")
						timer = time.Now()
					}
					session.RtpAudio.Push(rtpBytes)
				}
			case session.aRTPControlChannel:
				// pack.Type = RTP_TYPE_AUDIOCONTROL
			case session.vRTPChannel:
				// pack.Type = RTP_TYPE_VIDEO
				if session.RtpVideo != nil {
					elapsed := time.Since(timer)
					if elapsed >= 30*time.Second {
						Println("Recv an video RTP package")
						timer = time.Now()
					}
					session.RtpVideo.Push(rtpBytes)
				}
			case session.vRTPControlChannel:
				// pack.Type = RTP_TYPE_VIDEOCONTROL
			default:
				//	Printf("unknow rtp pack type, %v", pack.Type)
				continue
			}
			session.InBytes += rtpLen + 4
		} else { // rtsp cmd
			reqBuf := bytes.NewBuffer(nil)
			reqBuf.WriteByte(buf1)
			for {
				if line, isPrefix, err := session.connRW.ReadLine(); err != nil {
					Println(err)
					return
				} else {
					reqBuf.Write(line)
					if !isPrefix {
						reqBuf.WriteString("\r\n")
					}
					if len(line) == 0 {
						req := NewRequest(reqBuf.String())
						if req == nil {
							break
						}
						session.InBytes += reqBuf.Len()
						contentLen := req.GetContentLength()
						session.InBytes += contentLen
						if contentLen > 0 {
							bodyBuf := make([]byte, contentLen)
							if n, err := io.ReadFull(session.connRW, bodyBuf); err != nil {
								Println(err)
								return
							} else if n != contentLen {
								Printf("read rtsp request body failed, expect size[%d], got size[%d]", contentLen, n)
								return
							}
							req.Body = string(bodyBuf)
						}
						session.handleRequest(req)
						break
					}
				}
			}
		}
	}
}

func (session *RTSP) CheckAuth(authLine string, method string) error {
	realmRex := regexp.MustCompile(`realm="(.*?)"`)
	nonceRex := regexp.MustCompile(`nonce="(.*?)"`)
	usernameRex := regexp.MustCompile(`username="(.*?)"`)
	responseRex := regexp.MustCompile(`response="(.*?)"`)
	uriRex := regexp.MustCompile(`uri="(.*?)"`)

	realm := ""
	nonce := ""
	username := ""
	response := ""
	uri := ""
	result1 := realmRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		realm = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : no realm found")
	}
	result1 = nonceRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		nonce = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : no nonce found")
	}
	if session.nonce != nonce {
		return fmt.Errorf("CheckAuth error : sessionNonce not same as nonce")
	}

	result1 = usernameRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		username = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : username not found")
	}

	result1 = responseRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		response = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : response not found")
	}

	result1 = uriRex.FindStringSubmatch(authLine)
	if len(result1) == 2 {
		uri = result1[1]
	} else {
		return fmt.Errorf("CheckAuth error : uri not found")
	}
	// var user models.User
	// err := db.SQLite.Where("Username = ?", username).First(&user).Error
	// if err != nil {
	// 	return fmt.Errorf("CheckAuth error : user not exists")
	// }
	md5UserRealmPwd := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, session.Auth(username)))))
	md5MethodURL := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s", method, uri))))
	myResponse := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", md5UserRealmPwd, nonce, md5MethodURL))))
	if myResponse != response {
		return fmt.Errorf("CheckAuth error : response not equal")
	}
	return nil
}

func (session *RTSP) handleRequest(req *Request) {
	//if session.Timeout > 0 {
	//	session.Conn.SetDeadline(time.Now().Add(time.Duration(session.Timeout) * time.Second))
	//}
	Printf("<<<\n%s", req)
	res := NewResponse(200, "OK", req.Header["CSeq"], session.ID, "")
	var streamPath string
	defer func() {
		if p := recover(); p != nil {
			Printf("handleRequest err ocurs:%v", p)
			res.StatusCode = 500
			res.Status = fmt.Sprintf("Inner Server Error, %v", p)
		}
		Printf(">>>\n%s", res)
		outBytes := []byte(res.String())
		session.connWLock.Lock()
		session.connRW.Write(outBytes)
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += len(outBytes)
		switch req.Method {
		case "PLAY", "RECORD":
			switch session.Type {
			case SESSEION_TYPE_PLAYER:
				sub := Subscriber{
					ID:   session.ID,
					Type: "RTSP",
				}
				if sub.Subscribe(streamPath) == nil {
					at, vt := session.UDPClient.AT, session.UDPClient.VT
					if vt != nil {
						var st uint32
						onVideo := func(ts uint32, pack *VideoPack) {
							if session.UDPClient == nil {
								return
							}
							for _, nalu := range pack.NALUs {
								for _, pack := range session.UDPClient.VPacketizer.Packetize(nalu, (ts-st)*90) {
									p := &RTPPack{
										Type:   RTP_TYPE_VIDEO,
										Packet: *pack,
									}
									p.Raw, _ = p.Marshal()
									session.SendRTP(p)
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
					}
					if at != nil {
						tb := uint32(at.SoundRate / 1000)
						var st uint32
						onAudio := func(ts uint32, pack *AudioPack) {
							if session.UDPClient == nil {
								return
							}
							for _, pack := range session.UDPClient.APacketizer.Packetize(pack.Payload, (ts-st)*tb) {
								p := &RTPPack{
									Type:   RTP_TYPE_VIDEO,
									Packet: *pack,
								}
								p.Raw, _ = p.Marshal()
								session.SendRTP(p)
							}
							st = ts
						}
						sub.OnAudio = func(ts uint32, pack *AudioPack) {
							if st = ts; st != 0 {
								sub.OnAudio = onAudio
							}
							onAudio(ts, pack)
						}
					}
					go sub.Play(at, vt)
				}
				// if session.Pusher.HasPlayer(session.Player) {
				// 	session.Player.Pause(false)
				// } else {
				// 	session.Pusher.AddPlayer(session.Player)
				// }
			}
		case "TEARDOWN":
			{
				session.Stop()
				return
			}
		}
		if res.StatusCode != 200 && res.StatusCode != 401 {
			Printf("Response request error[%d]. stop session.", res.StatusCode)
			session.Stop()
		}
	}()
	session.URL = req.URL
	_url, err := url.Parse(req.URL)
	if err != nil {
		res.StatusCode = 500
		res.Status = "Invalid URL"
		return
	}
	streamPath = strings.TrimPrefix(_url.Path, "/")
	if req.Method != "OPTIONS" {
		if session.Auth != nil {
			authLine := req.Header["Authorization"]
			authFailed := true
			if authLine != "" {
				err := session.CheckAuth(authLine, req.Method)
				if err == nil {
					authFailed = false
				} else {
					Printf("%v", err)
				}
			}
			if authFailed {
				res.StatusCode = 401
				res.Status = "Unauthorized"
				nonce := fmt.Sprintf("%x", md5.Sum([]byte(shortid.MustGenerate())))
				session.nonce = nonce
				res.Header["WWW-Authenticate"] = fmt.Sprintf(`Digest realm="Monibuca", nonce="%s", algorithm="MD5"`, nonce)
				return
			}
		}
	}
	switch req.Method {
	case "OPTIONS":
		res.Header["Public"] = "DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE, OPTIONS, ANNOUNCE, RECORD"
	case "ANNOUNCE":
		session.Type = SESSION_TYPE_PUSHER
		session.SDPRaw = req.Body
		session.SDPMap = ParseSDP(req.Body)
		if session.Stream = Publish(streamPath, "RTSP"); session.Stream != nil {
			if session.ASdp, session.HasAudio = session.SDPMap["audio"]; session.HasAudio {
				session.setAudioTrack()
				Printf("audio codec[%s]\n", session.ASdp.Codec)
			}
			if session.VSdp, session.HasVideo = session.SDPMap["video"]; session.HasVideo {
				session.setVideoTrack()
				Printf("video codec[%s]\n", session.VSdp.Codec)
			}
			session.Stream.Type = "RTSP"
		}
	case "DESCRIBE":
		session.Type = SESSEION_TYPE_PLAYER
		stream := FindStream(streamPath)
		if stream == nil {
			res.StatusCode = 404
			res.Status = "No Such Stream:" + streamPath
			return
		}
		sdpInfo := []string{
			"v=0",
			fmt.Sprintf("o=%s 0 0 IN IP4 %d", session.ID, 0),
			"s=monibuca",
			"t=0 0",
			"a=recvonly",
		}
		ssrc := uintptr(unsafe.Pointer(stream))
		if session.UDPClient == nil {
			session.UDPClient = &UDPClient{
				Conn: session.Conn.Conn,
			}
		}
		vt, at := stream.WaitVideoTrack(), stream.WaitAudioTrack()
		if vt != nil {
			session.UDPClient.VT = vt
			sdpInfo = append(sdpInfo, "m=video 0 RTP/AVP 96")
			switch vt.CodecID {
			case 7:
				sps := base64.StdEncoding.EncodeToString(vt.ExtraData.NALUs[0])
				pps := base64.StdEncoding.EncodeToString(vt.ExtraData.NALUs[1])
				session.UDPClient.VPacketizer = rtp.NewPacketizer(1200, 96, uint32(ssrc), &codecs.H264Payloader{}, rtp.NewFixedSequencer(1), 90000)
				sdpInfo = append(sdpInfo, "a=rtpmap:96 H264/90000",
					fmt.Sprintf("a=fmtp:96 profile-level-id=%02X00%02X; packetization-mode=1; sprop-parameter-sets=%s,%s", vt.SPSInfo.ProfileIdc, vt.SPSInfo.LevelIdc*10, sps, pps))
			case 12:
				vps := base64.StdEncoding.EncodeToString(vt.ExtraData.NALUs[0])
				sps := base64.StdEncoding.EncodeToString(vt.ExtraData.NALUs[1])
				pps := base64.StdEncoding.EncodeToString(vt.ExtraData.NALUs[2])
				// TODO:
				// session.UDPClient.VPacketizer = rtp.NewPacketizer(1200, 96, uint32(ssrc), &codecs.H265Payloader{}, rtp.NewFixedSequencer(1), 90000)
				sdpInfo = append(sdpInfo, "a=rtpmap:96 H265/90000",
					fmt.Sprintf("a=fmtp:96 packetization-mode=1;sprop-vps=%s;sprop-sps=%s;sprop-pps=%s", vps, sps, pps))
			}
		}
		if at != nil {
			sdpInfo = append(sdpInfo, "m=audio 0 RTP/AVP 97")
			switch at.CodecID {
			case 7:
				sdpInfo = append(sdpInfo, "a=rtpmap:97 PCMA/8000")
				session.UDPClient.APacketizer = rtp.NewPacketizer(1200, 97, uint32(ssrc), &codecs.G711Payloader{}, rtp.NewFixedSequencer(1), 8000)
				session.UDPClient.AT = at
			case 8:
				sdpInfo = append(sdpInfo, "a=rtpmap:97 PCMU/8000")
				session.UDPClient.APacketizer = rtp.NewPacketizer(1200, 97, uint32(ssrc), &codecs.G711Payloader{}, rtp.NewFixedSequencer(1), 8000)
				session.UDPClient.AT = at
			case 10:
				// TODO:
				sdpInfo = append(sdpInfo, fmt.Sprintf("a=rtpmap:97 MPEG4-GENERIC/%d/%d", at.SoundRate, at.Channels))
			}
		}
		session.SDPRaw = strings.Join(sdpInfo, "\r\n") + "\r\n"
		res.SetBody(session.SDPRaw)
	case "SETUP":
		ts := req.Header["Transport"]
		// control字段可能是`stream=1`字样，也可能是rtsp://...字样。即control可能是url的path，也可能是整个url
		// 例1：
		// a=control:streamid=1
		// 例2：
		// a=control:rtsp://192.168.1.64/trackID=1
		// 例3：
		// a=control:?ctype=video
		if _url.Port() == "" {
			_url.Host = fmt.Sprintf("%s:554", _url.Host)
		}
		setupPath := _url.String()

		// error status. SETUP without ANNOUNCE or DESCRIBE.
		//if session.Pusher == nil {
		//	res.StatusCode = 500
		//	res.Status = "Error Status"
		//	return
		//}
		var vPath, aPath string
		if session.HasVideo {
			if strings.Index(strings.ToLower(session.VSdp.Control), "rtsp://") == 0 {
				vControlUrl, err := url.Parse(session.VSdp.Control)
				if err != nil {
					res.StatusCode = 500
					res.Status = "Invalid VControl"
					return
				}
				if vControlUrl.Port() == "" {
					vControlUrl.Host = fmt.Sprintf("%s:554", vControlUrl.Host)
				}
				vPath = vControlUrl.String()
			} else {
				vPath = session.VSdp.Control
			}
		}
		if session.HasAudio {
			if strings.Index(strings.ToLower(session.ASdp.Control), "rtsp://") == 0 {
				aControlUrl, err := url.Parse(session.ASdp.Control)
				if err != nil {
					res.StatusCode = 500
					res.Status = "Invalid AControl"
					return
				}
				if aControlUrl.Port() == "" {
					aControlUrl.Host = fmt.Sprintf("%s:554", aControlUrl.Host)
				}
				aPath = aControlUrl.String()
			} else {
				aPath = session.ASdp.Control
			}
		}

		mtcp := regexp.MustCompile("interleaved=(\\d+)(-(\\d+))?")
		mudp := regexp.MustCompile("client_port=(\\d+)(-(\\d+))?")

		if tcpMatchs := mtcp.FindStringSubmatch(ts); tcpMatchs != nil {
			session.TransType = TRANS_TYPE_TCP
			if setupPath == aPath || aPath != "" && strings.LastIndex(setupPath, aPath) == len(setupPath)-len(aPath) {
				session.aRTPChannel, _ = strconv.Atoi(tcpMatchs[1])
				session.aRTPControlChannel, _ = strconv.Atoi(tcpMatchs[3])
			} else if setupPath == vPath || vPath != "" && strings.LastIndex(setupPath, vPath) == len(setupPath)-len(vPath) {
				session.vRTPChannel, _ = strconv.Atoi(tcpMatchs[1])
				session.vRTPControlChannel, _ = strconv.Atoi(tcpMatchs[3])
			} else {
				res.StatusCode = 500
				res.Status = fmt.Sprintf("SETUP [TCP] got UnKown control:%s", setupPath)
				Printf("SETUP [TCP] got UnKown control:%s", setupPath)
			}
			Printf("Parse SETUP req.TRANSPORT:TCP.Session.Type:%d,control:%s, AControl:%s,VControl:%s", session.Type, setupPath, aPath, vPath)
		} else if udpMatchs := mudp.FindStringSubmatch(ts); udpMatchs != nil {
			session.TransType = TRANS_TYPE_UDP
			// no need for tcp timeout.
			session.Conn.timeout = 0
			if session.Type == SESSEION_TYPE_PLAYER && session.UDPClient == nil {
				session.UDPClient = &UDPClient{}
			}
			if session.Type == SESSION_TYPE_PUSHER && session.UDPServer == nil {
				session.UDPServer = &UDPServer{
					Session: session,
				}
			}
			Printf("Parse SETUP req.TRANSPORT:UDP.Session.Type:%d,control:%s, AControl:%s,VControl:%s", session.Type, setupPath, aPath, vPath)
			if setupPath == aPath || aPath != "" && strings.LastIndex(setupPath, aPath) == len(setupPath)-len(aPath) {
				if session.Type == SESSEION_TYPE_PLAYER {
					session.UDPClient.APort, _ = strconv.Atoi(udpMatchs[1])
					session.UDPClient.AControlPort, _ = strconv.Atoi(udpMatchs[3])
					if err := session.UDPClient.SetupAudio(); err != nil {
						res.StatusCode = 500
						res.Status = fmt.Sprintf("udp client setup audio error, %v", err)
						return
					}
				}
				if session.Type == SESSION_TYPE_PUSHER {
					if err := session.UDPServer.SetupAudio(); err != nil {
						res.StatusCode = 500
						res.Status = fmt.Sprintf("udp server setup audio error, %v", err)
						return
					}
					tss := strings.Split(ts, ";")
					idx := -1
					for i, val := range tss {
						if val == udpMatchs[0] {
							idx = i
						}
					}
					tail := append([]string{}, tss[idx+1:]...)
					tss = append(tss[:idx+1], fmt.Sprintf("server_port=%d-%d", session.UDPServer.APort, session.UDPServer.AControlPort))
					tss = append(tss, tail...)
					ts = strings.Join(tss, ";")
				}
			} else if setupPath == vPath || vPath != "" && strings.LastIndex(setupPath, vPath) == len(setupPath)-len(vPath) {
				if session.Type == SESSEION_TYPE_PLAYER {
					session.UDPClient.VPort, _ = strconv.Atoi(udpMatchs[1])
					session.UDPClient.VControlPort, _ = strconv.Atoi(udpMatchs[3])
					if err := session.UDPClient.SetupVideo(); err != nil {
						res.StatusCode = 500
						res.Status = fmt.Sprintf("udp client setup video error, %v", err)
						return
					}
				}

				if session.Type == SESSION_TYPE_PUSHER {
					if err := session.UDPServer.SetupVideo(); err != nil {
						res.StatusCode = 500
						res.Status = fmt.Sprintf("udp server setup video error, %v", err)
						return
					}
					tss := strings.Split(ts, ";")
					idx := -1
					for i, val := range tss {
						if val == udpMatchs[0] {
							idx = i
						}
					}
					tail := append([]string{}, tss[idx+1:]...)
					tss = append(tss[:idx+1], fmt.Sprintf("server_port=%d-%d", session.UDPServer.VPort, session.UDPServer.VControlPort))
					tss = append(tss, tail...)
					ts = strings.Join(tss, ";")
				}
			} else {
				if session.Type == SESSEION_TYPE_PLAYER {
					if session.UDPClient.VPort == 0 {
						session.UDPClient.VPort, _ = strconv.Atoi(udpMatchs[1])
						session.UDPClient.VControlPort, _ = strconv.Atoi(udpMatchs[3])
						if err := session.UDPClient.SetupVideo(); err != nil {
							res.StatusCode = 500
							res.Status = fmt.Sprintf("udp client setup video error, %v", err)
							return
						}
					} else {
						session.UDPClient.APort, _ = strconv.Atoi(udpMatchs[1])
						session.UDPClient.AControlPort, _ = strconv.Atoi(udpMatchs[3])
						if err := session.UDPClient.SetupAudio(); err != nil {
							res.StatusCode = 500
							res.Status = fmt.Sprintf("udp client setup audio error, %v", err)
							return
						}
					}
				}
				Printf("SETUP [UDP] got UnKown control:%s", setupPath)
			}
		}
		res.Header["Transport"] = ts
	case "PLAY":
		// error status. PLAY without ANNOUNCE or DESCRIBE.
		// if session.Pusher == nil {
		// 	res.StatusCode = 500
		// 	res.Status = "Error Status"
		// 	return
		// }
		res.Header["Range"] = req.Header["Range"]
	case "RECORD":
		// error status. RECORD without ANNOUNCE or DESCRIBE.
		// if session.Pusher == nil {
		// 	res.StatusCode = 500
		// 	res.Status = "Error Status"
		// 	return
		// }
	case "PAUSE":
		// if session.Player == nil {
		// 	res.StatusCode = 500
		// 	res.Status = "Error Status"
		// 	return
		// }
		// session.Player.Pause(true)
	}
}

func (session *RTSP) SendRTP(pack *RTPPack) (err error) {
	if pack == nil {
		err = fmt.Errorf("player send rtp got nil pack")
		return
	}
	if session.TransType == TRANS_TYPE_UDP {
		if session.UDPClient == nil {
			err = fmt.Errorf("player use udp transport but udp client not found")
			return
		}
		err = session.UDPClient.SendRTP(pack)
		session.OutBytes += len(pack.Raw)
		return
	}
	switch pack.Type {
	case RTP_TYPE_AUDIO:
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(session.aRTPChannel)
		session.connWLock.Lock()
		session.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(len(pack.Raw)))
		session.connRW.Write(bufLen)
		session.connRW.Write(pack.Raw)
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += len(pack.Raw) + 4
	case RTP_TYPE_AUDIOCONTROL:
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(session.aRTPControlChannel)
		session.connWLock.Lock()
		session.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(len(pack.Raw)))
		session.connRW.Write(bufLen)
		session.connRW.Write(pack.Raw)
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += len(pack.Raw) + 4
	case RTP_TYPE_VIDEO:
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(session.vRTPChannel)
		session.connWLock.Lock()
		session.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(len(pack.Raw)))
		session.connRW.Write(bufLen)
		session.connRW.Write(pack.Raw)
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += len(pack.Raw) + 4
	case RTP_TYPE_VIDEOCONTROL:
		bufChannel := make([]byte, 2)
		bufChannel[0] = 0x24
		bufChannel[1] = byte(session.vRTPControlChannel)
		session.connWLock.Lock()
		session.connRW.Write(bufChannel)
		bufLen := make([]byte, 2)
		binary.BigEndian.PutUint16(bufLen, uint16(len(pack.Raw)))
		session.connRW.Write(bufLen)
		session.connRW.Write(pack.Raw)
		session.connRW.Flush()
		session.connWLock.Unlock()
		session.OutBytes += len(pack.Raw) + 4
	default:
		err = fmt.Errorf("session tcp send rtp got unkown pack type[%v]", pack.Type)
	}
	return
}
