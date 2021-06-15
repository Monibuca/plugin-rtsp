package rtsp

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
)

// PullStream 从外部拉流
func (rtsp *RTSP) PullStream(streamPath string, rtspUrl string) (err error) {
	rtsp.Stream = &Stream{
		StreamPath: streamPath,
		Type:       "RTSP Pull",
	}
	if result := rtsp.Publish(); result {
		rtsp.TransType = TRANS_TYPE_TCP
		rtsp.vRTPChannel = 0
		rtsp.vRTPControlChannel = 1
		rtsp.aRTPChannel = 2
		rtsp.aRTPControlChannel = 3
		rtsp.URL = rtspUrl
		rtsp.UDPServer = &UDPServer{Session: rtsp}
		if err = rtsp.requestStream(); err != nil {
			Println(err)
			rtsp.Close()
			return
		}
		go rtsp.startStream()
		collection.Store(streamPath, rtsp)
		return
	}
	return errors.New("publish badname")
}
func DigestAuth(authLine string, method string, URL string) (string, error) {
	l, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("Url parse error:%v,%v", URL, err)
	}
	realm := ""
	nonce := ""
	realmRex := regexp.MustCompile(`realm="(.*?)"`)
	result1 := realmRex.FindStringSubmatch(authLine)

	nonceRex := regexp.MustCompile(`nonce="(.*?)"`)
	result2 := nonceRex.FindStringSubmatch(authLine)

	if len(result1) == 2 {
		realm = result1[1]
	} else {
		return "", fmt.Errorf("auth error : no realm found")
	}
	if len(result2) == 2 {
		nonce = result2[1]
	} else {
		return "", fmt.Errorf("auth error : no nonce found")
	}
	// response= md5(md5(username:realm:password):nonce:md5(public_method:url));
	username := l.User.Username()
	password, _ := l.User.Password()
	l.User = nil
	if l.Port() == "" {
		l.Host = fmt.Sprintf("%s:%s", l.Host, "554")
	}
	md5UserRealmPwd := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", username, realm, password))))
	md5MethodURL := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s", method, l.String()))))

	response := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", md5UserRealmPwd, nonce, md5MethodURL))))
	Authorization := fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", response=\"%s\"", username, realm, nonce, l.String(), response)
	return Authorization, nil
}

// auth Basic验证
func BasicAuth(authLine string, method string, URL string) (string, error) {
	l, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("Url parse error:%v,%v", URL, err)
	}
	username := l.User.Username()
	password, _ := l.User.Password()
	userAndpass := []byte(username + ":" + password)
	Authorization := fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString(userAndpass))
	return Authorization, nil
}
func (client *RTSP) checkAuth(method string, resp *Response) (string, error) {
	if resp.StatusCode == 401 {
		// need auth.
		AuthHeaders := resp.Header["WWW-Authenticate"]
		auths, ok := AuthHeaders.([]string)
		if ok {
			for _, authLine := range auths {
				if strings.IndexAny(authLine, "Digest") == 0 {
					// 					realm="HipcamRealServer",
					// nonce="3b27a446bfa49b0c48c3edb83139543d"
					client.authLine = authLine
					return DigestAuth(authLine, method, client.URL)
				} else if strings.IndexAny(authLine, "Basic") == 0 {
					return BasicAuth(authLine, method, client.URL)
				}
			}
			return "", fmt.Errorf("auth error")
		} else {
			authLine, _ := AuthHeaders.(string)
			if strings.IndexAny(authLine, "Digest") == 0 {
				client.authLine = authLine
				return DigestAuth(authLine, method, client.URL)
			} else if strings.IndexAny(authLine, "Basic") == 0 {
				return BasicAuth(authLine, method, client.URL)
			}
		}
	}
	return "", nil
}
func (client *RTSP) requestStream() (err error) {
	timeout := time.Duration(5) * time.Second
	l, err := url.Parse(client.URL)
	if err != nil {
		return err
	}
	if strings.ToLower(l.Scheme) != "rtsp" {
		err = fmt.Errorf("RTSP url is invalid")
		return err
	}
	if strings.ToLower(l.Hostname()) == "" {
		err = fmt.Errorf("RTSP url is invalid")
		return err
	}
	port := l.Port()
	if len(port) == 0 {
		port = "554"
	}
	conn, err := net.DialTimeout("tcp", l.Hostname()+":"+port, timeout)
	if err != nil {
		// handle error
		return err
	}

	networkBuffer := 204800

	timeoutConn := RichConn{
		conn,
		timeout,
	}
	client.Conn = &timeoutConn
	client.connRW = bufio.NewReadWriter(bufio.NewReaderSize(&timeoutConn, networkBuffer), bufio.NewWriterSize(&timeoutConn, networkBuffer))

	headers := make(map[string]string)
	//headers["Require"] = "implicit-play"
	// An OPTIONS request returns the request types the server will accept.
	resp, err := client.Request("OPTIONS", headers)
	if err != nil {
		if resp != nil {
			Authorization, _ := client.checkAuth("OPTIONS", resp)
			if len(Authorization) > 0 {
				headers := make(map[string]string)
				headers["Require"] = "implicit-play"
				headers["Authorization"] = Authorization
				// An OPTIONS request returns the request types the server will accept.
				resp, err = client.Request("OPTIONS", headers)
			}
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// A DESCRIBE request includes an RTSP URL (rtsp://...), and the type of reply data that can be handled. This reply includes the presentation description,
	// typically in Session Description Protocol (SDP) format. Among other things, the presentation description lists the media streams controlled with the aggregate URL.
	// In the typical case, there is one media stream each for audio and video.
	headers = make(map[string]string)
	headers["Accept"] = "application/sdp"
	resp, err = client.Request("DESCRIBE", headers)
	if err != nil {
		if resp != nil {
			authorization, _ := client.checkAuth("DESCRIBE", resp)
			if len(authorization) > 0 {
				headers := make(map[string]string)
				headers["Authorization"] = authorization
				headers["Accept"] = "application/sdp"
				resp, err = client.Request("DESCRIBE", headers)
			}
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	client.SDPRaw = resp.Body
	client.SDPMap = ParseSDP(client.SDPRaw)
	client.VSdp, client.HasVideo = client.SDPMap["video"]
	client.ASdp, client.HasAudio = client.SDPMap["audio"]
	session := ""
	otherChannel := 4
	for t, sdpInfo := range client.SDPMap {
		headers = make(map[string]string)
		if session != "" {
			headers["Session"] = session
		}
		var _url = sdpInfo.Control
		if !strings.HasPrefix(strings.ToLower(sdpInfo.Control), "rtsp://") {
			_url = strings.TrimRight(client.URL, "/") + "/" + strings.TrimLeft(sdpInfo.Control, "/")
		}
		switch t {
		case "video":
			if len(sdpInfo.SpropParameterSets) > 1 {
				client.RtpVideo = client.NewRTPVideo(7)
				client.RtpVideo.PushNalu(VideoPack{NALUs: sdpInfo.SpropParameterSets})
			} else if client.VSdp.Codec == "H264" {
				client.RtpVideo = client.NewRTPVideo(7)
			} else if client.VSdp.Codec == "H265" {
				client.RtpVideo = client.NewRTPVideo(12)
			}
			if client.TransType == TRANS_TYPE_TCP {
				headers["Transport"] = fmt.Sprintf("RTP/AVP/TCP;unicast;interleaved=%d-%d", client.vRTPChannel, client.vRTPControlChannel)
			} else {
				//RTP/AVP;unicast;client_port=64864-64865
				if err = client.UDPServer.SetupVideo(); err != nil {
					Printf("Setup video err.%v", err)
					return err
				}
				headers["Transport"] = fmt.Sprintf("RTP/AVP/UDP;unicast;client_port=%d-%d", client.UDPServer.VPort, client.UDPServer.VControlPort)
				client.Conn.timeout = 0 //	UDP ignore timeout
			}
		case "audio":
			client.RtpAudio = client.NewRTPAudio(0)
			at := client.RtpAudio.AudioTrack
			if len(client.ASdp.Control) > 0 {
				at.SetASC(client.ASdp.Config)
			} else {
				client.setAudioFormat(at)
			}
			if client.TransType == TRANS_TYPE_TCP {
				headers["Transport"] = fmt.Sprintf("RTP/AVP/TCP;unicast;interleaved=%d-%d", client.aRTPChannel, client.aRTPControlChannel)
			} else {
				if err = client.UDPServer.SetupAudio(); err != nil {
					Printf("Setup audio err.%v", err)
					return err
				}
				headers["Transport"] = fmt.Sprintf("RTP/AVP/UDP;unicast;client_port=%d-%d", client.UDPServer.APort, client.UDPServer.AControlPort)
				client.Conn.timeout = 0 //	UDP ignore timeout
			}
		default:
			if client.TransType == TRANS_TYPE_TCP {
				headers["Transport"] = fmt.Sprintf("RTP/AVP/TCP;unicast;interleaved=%d-%d", otherChannel, otherChannel+1)
				otherChannel += 2
			} else {
				//TODO: UDP support
			}
		}
		if resp, err = client.RequestWithPath("SETUP", _url, headers, true); err != nil {
			return err
		}
		session, _ = resp.Header["Session"].(string)
		session = strings.Split(session, ";")[0]
	}
	headers = make(map[string]string)
	if session != "" {
		headers["Session"] = session
		client.Session = session
	}
	resp, err = client.Request("PLAY", headers)
	return err
}

func (client *RTSP) startStream() {
	startTime := time.Now()
	//loggerTime := time.Now().Add(-10 * time.Second)
	defer func() {
		if client.Err() == nil && config.Reconnect {
			Printf("reconnecting:%s", client.URL)
			client.RTSPClientInfo = RTSPClientInfo{}
			if err := client.requestStream(); err != nil {
				t := time.NewTicker(time.Second * 5)
				for {
					Printf("reconnecting:%s in 5 seconds", client.URL)
					select {
					case <-client.Done():
						client.Stop()
						return
					case <-t.C:
						if err = client.requestStream(); err == nil {
							go client.startStream()
							return
						}
					}
				}
			} else {
				go client.startStream()
			}
		} else {
			client.Stop()
		}
	}()
	for client.Err() == nil {
		if time.Since(startTime) > time.Minute {
			startTime = time.Now()
			headers := make(map[string]string)
			headers["Require"] = "implicit-play"
			// An OPTIONS request returns the request types the server will accept.
			if err := client.RequestNoResp("GET_PARAMETER", headers); err != nil {
				// ignore...
			}
		}
		b, err := client.connRW.ReadByte()
		if err != nil {
			Printf("client.connRW.ReadByte err:%v", err)
			return
		}
		switch b {
		case 0x24: // rtp
			header := make([]byte, 4)
			header[0] = b
			_, err := io.ReadFull(client.connRW, header[1:])
			if err != nil {
				Printf("io.ReadFull err:%v", err)
				return
			}
			channel := int(header[1])
			length := binary.BigEndian.Uint16(header[2:])
			content := make([]byte, length)
			_, err = io.ReadFull(client.connRW, content)
			if err != nil {
				Printf("io.ReadFull err:%v", err)
				return
			}
			
			switch channel {
			case client.aRTPChannel:
				client.RtpAudio.Push(content)
			case client.aRTPControlChannel:

			case client.vRTPChannel:
				client.RtpVideo.Push(content)
			case client.vRTPControlChannel:

			default:
				Printf("unknow rtp pack type, channel:%v", channel)
				continue
			}

			//if client.debugLogEnable {
			//	rtp := ParseRTP(pack.Buffer)
			//	if rtp != nil {
			//		rtpSN := uint16(rtp.SequenceNumber)
			//		if client.lastRtpSN != 0 && client.lastRtpSN+1 != rtpSN {
			//			Printf("%s, %d packets lost, current SN=%d, last SN=%d\n", client.String(), rtpSN-client.lastRtpSN, rtpSN, client.lastRtpSN)
			//		}
			//		client.lastRtpSN = rtpSN
			//	}
			//
			//	elapsed := time.Now().Sub(loggerTime)
			//	if elapsed >= 30*time.Second {
			//		Printf("%v read rtp frame.", client)
			//		loggerTime = time.Now()
			//	}
			//}

			client.InBytes += int(length + 4)

		default: // rtsp
			builder := bytes.Buffer{}
			builder.WriteByte(b)
			contentLen := 0
			for client.Err() == nil {
				line, prefix, err := client.connRW.ReadLine()
				if err != nil {
					Printf("client.connRW.ReadLine err:%v", err)
					return
				}
				if len(line) == 0 {
					if contentLen != 0 {
						content := make([]byte, contentLen)
						_, err = io.ReadFull(client.connRW, content)
						if err != nil {
							err = fmt.Errorf("Read content err.ContentLength:%d", contentLen)
							return
						}
						builder.Write(content)
					}
					Printf("<<<[IN]\n%s", builder.String())
					break
				}
				s := string(line)
				builder.Write(line)
				if !prefix {
					builder.WriteString("\r\n")
				}

				if strings.Index(s, "Content-Length:") == 0 {
					splits := strings.Split(s, ":")
					contentLen, err = strconv.Atoi(strings.TrimSpace(splits[1]))
					if err != nil {
						Printf("strconv.Atoi err:%v, str:%v", err, splits[1])
						return
					}
				}
			}
		}
	}
}

func (client *RTSP) Request(method string, headers map[string]string) (*Response, error) {
	l, err := url.Parse(client.URL)
	if err != nil {
		return nil, fmt.Errorf("Url parse error:%v", err)
	}
	l.User = nil
	return client.RequestWithPath(method, l.String(), headers, true)
}

func (client *RTSP) RequestNoResp(method string, headers map[string]string) (err error) {
	l, err := url.Parse(client.URL)
	if err != nil {
		return fmt.Errorf("Url parse error:%v", err)
	}
	l.User = nil
	if _, err = client.RequestWithPath(method, l.String(), headers, false); err != nil {
		return err
	}
	return nil
}

func (client *RTSP) RequestWithPath(method string, path string, headers map[string]string, needResp bool) (resp *Response, err error) {
	headers["User-Agent"] = client.Agent
	if len(headers["Authorization"]) == 0 {
		if len(client.authLine) != 0 {
			Authorization, _ := DigestAuth(client.authLine, method, client.URL)
			if len(Authorization) > 0 {
				headers["Authorization"] = Authorization
			}
		}
	}
	if len(client.Session) > 0 {
		headers["Session"] = client.Session
	}
	client.Seq++
	cseq := client.Seq
	builder := bytes.Buffer{}
	builder.WriteString(fmt.Sprintf("%s %s RTSP/1.0\r\n", method, path))
	builder.WriteString(fmt.Sprintf("CSeq: %d\r\n", cseq))
	for k, v := range headers {
		builder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	builder.WriteString(fmt.Sprintf("\r\n"))
	s := builder.String()
	Printf("[OUT]>>>\n%s", s)
	_, err = client.connRW.WriteString(s)
	if err != nil {
		return
	}
	client.connRW.Flush()

	if !needResp {
		return nil, nil
	}
	lineCount := 0
	statusCode := 200
	status := ""
	sid := ""
	contentLen := 0
	respHeader := make(map[string]interface{})
	var line []byte
	builder.Reset()
	for {
		isPrefix := false
		if line, isPrefix, err = client.connRW.ReadLine(); err != nil {
			return
		}
		s := string(line)
		builder.Write(line)
		if !isPrefix {
			builder.WriteString("\r\n")
		}
		if len(line) == 0 {
			body := ""
			if contentLen > 0 {
				content := make([]byte, contentLen)
				_, err = io.ReadFull(client.connRW, content)
				if err != nil {
					err = fmt.Errorf("Read content err.ContentLength:%d", contentLen)
					return
				}
				body = string(content)
				builder.Write(content)
			}
			resp = NewResponse(statusCode, status, strconv.Itoa(cseq), sid, body)
			resp.Header = respHeader
			Printf("<<<[IN]\n%s", builder.String())

			if !(statusCode >= 200 && statusCode <= 300) {
				err = fmt.Errorf("Response StatusCode is :%d", statusCode)
				return
			}
			return
		}
		if lineCount == 0 {
			splits := strings.Split(s, " ")
			if len(splits) < 3 {
				err = fmt.Errorf("StatusCode Line error:%s", s)
				return
			}
			statusCode, err = strconv.Atoi(splits[1])
			if err != nil {
				return
			}
			status = splits[2]
		}
		lineCount++
		splits := strings.Split(s, ":")
		if len(splits) == 2 {
			if val, ok := respHeader[splits[0]]; ok {
				if slice, ok2 := val.([]string); ok2 {
					slice = append(slice, strings.TrimSpace(splits[1]))
					respHeader[splits[0]] = slice
				} else {
					str, _ := val.(string)
					slice := []string{str, strings.TrimSpace(splits[1])}
					respHeader[splits[0]] = slice
				}
			} else {
				respHeader[splits[0]] = strings.TrimSpace(splits[1])
			}
		}
		if strings.Index(s, "Session:") == 0 {
			splits := strings.Split(s, ":")
			sid = strings.TrimSpace(splits[1])
		}
		//if strings.Index(s, "CSeq:") == 0 {
		//	splits := strings.Split(s, ":")
		//	cseq, err = strconv.Atoi(strings.TrimSpace(splits[1]))
		//	if err != nil {
		//		err = fmt.Errorf("Atoi CSeq err. line:%s", s)
		//		return
		//	}
		//}
		if strings.Index(s, "Content-Length:") == 0 {
			splits := strings.Split(s, ":")
			contentLen, err = strconv.Atoi(strings.TrimSpace(splits[1]))
			if err != nil {
				return
			}
		}

	}
	return
}
