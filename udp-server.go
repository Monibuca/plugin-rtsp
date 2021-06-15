package rtsp

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	. "github.com/Monibuca/utils/v3"
)

type UDPServer struct {
	Session *RTSP
	UDPClient
	sync.Mutex
}

func (s *UDPServer) AddInputBytes(bytes int) {
	if s.Session != nil {
		s.Session.InBytes += bytes
		return
	}
	panic(fmt.Errorf("session and RTSPClient both nil"))
}

func (s *UDPServer) Stop() {
	if s.Stoped {
		return
	}
	s.Stoped = true
	if s.AConn != nil {
		s.AConn.Close()
		s.AConn = nil
	}
	if s.AControlConn != nil {
		s.AControlConn.Close()
		s.AControlConn = nil
	}
	if s.VConn != nil {
		s.VConn.Close()
		s.VConn = nil
	}
	if s.VControlConn != nil {
		s.VControlConn.Close()
		s.VControlConn = nil
	}
}

func (s *UDPServer) SetupAudio() (err error) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return
	}
	s.AConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	networkBuffer := 1048576
	if err := s.AConn.SetReadBuffer(networkBuffer); err != nil {
		Printf("udp server audio conn set read buffer error, %v", err)
	}
	if err := s.AConn.SetWriteBuffer(networkBuffer); err != nil {
		Printf("udp server audio conn set write buffer error, %v", err)
	}
	la := s.AConn.LocalAddr().String()
	strPort := la[strings.LastIndex(la, ":")+1:]
	s.APort, err = strconv.Atoi(strPort)
	if err != nil {
		return
	}
	go func() {
		bufUDP := make([]byte, UDP_BUF_SIZE)
		Printf("udp server start listen audio port[%d]", s.APort)
		defer Printf("udp server stop listen audio port[%d]", s.APort)
		timer := time.Unix(0, 0)
		for !s.Stoped {
			if n, _, err := s.AConn.ReadFromUDP(bufUDP); err == nil {
				elapsed := time.Now().Sub(timer)
				if elapsed >= 30*time.Second {
					Printf("Package recv from AConn.len:%d\n", n)
					timer = time.Now()
				}
				s.AddInputBytes(n)
				s.Session.RtpAudio.Push(bufUDP[:n])
			} else {
				Println("udp server read audio pack error", err)
				continue
			}
		}
	}()
	addr, err = net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return
	}
	s.AControlConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	if err := s.AControlConn.SetReadBuffer(networkBuffer); err != nil {
		Printf("udp server audio control conn set read buffer error, %v", err)
	}
	if err := s.AControlConn.SetWriteBuffer(networkBuffer); err != nil {
		Printf("udp server audio control conn set write buffer error, %v", err)
	}
	la = s.AControlConn.LocalAddr().String()
	strPort = la[strings.LastIndex(la, ":")+1:]
	s.AControlPort, err = strconv.Atoi(strPort)
	if err != nil {
		return
	}
	go func() {
		bufUDP := make([]byte, UDP_BUF_SIZE)
		Printf("udp server start listen audio control port[%d]", s.AControlPort)
		defer Printf("udp server stop listen audio control port[%d]", s.AControlPort)
		for !s.Stoped {
			if n, _, err := s.AControlConn.ReadFromUDP(bufUDP); err == nil {
				//Printf("Package recv from AControlConn.len:%d\n", n)
				s.AddInputBytes(n)
				// pack := RTPPack{
				// 	Type: RTP_TYPE_AUDIOCONTROL,
				// }
				// pack.Unmarshal(bufUDP[:n])
				// s.HandleRTP(pack)
			} else {
				Println("udp server read audio control pack error", err)
				continue
			}
		}
	}()
	return
}

func (s *UDPServer) SetupVideo() (err error) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return
	}
	s.VConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	networkBuffer := 1048576
	if err := s.VConn.SetReadBuffer(networkBuffer); err != nil {
		Printf("udp server video conn set read buffer error, %v", err)
	}
	if err := s.VConn.SetWriteBuffer(networkBuffer); err != nil {
		Printf("udp server video conn set write buffer error, %v", err)
	}
	la := s.VConn.LocalAddr().String()
	strPort := la[strings.LastIndex(la, ":")+1:]
	s.VPort, err = strconv.Atoi(strPort)
	if err != nil {
		return
	}
	go func() {
		bufUDP := make([]byte, UDP_BUF_SIZE)
		Printf("udp server start listen video port[%d]", s.VPort)
		defer Printf("udp server stop listen video port[%d]", s.VPort)
		timer := time.Unix(0, 0)
		for !s.Stoped {
			if n, _, err := s.VConn.ReadFromUDP(bufUDP); err == nil {
				elapsed := time.Now().Sub(timer)
				if elapsed >= 30*time.Second {
					Printf("Package recv from VConn.len:%d\n", n)
					timer = time.Now()
				}
				s.AddInputBytes(n)
				s.Session.RtpVideo.Push(bufUDP[:n])
			} else {
				Println("udp server read video pack error", err)
				continue
			}
		}
	}()

	addr, err = net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return
	}
	s.VControlConn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	if err := s.VControlConn.SetReadBuffer(networkBuffer); err != nil {
		Printf("udp server video control conn set read buffer error, %v", err)
	}
	if err := s.VControlConn.SetWriteBuffer(networkBuffer); err != nil {
		Printf("udp server video control conn set write buffer error, %v", err)
	}
	la = s.VControlConn.LocalAddr().String()
	strPort = la[strings.LastIndex(la, ":")+1:]
	s.VControlPort, err = strconv.Atoi(strPort)
	if err != nil {
		return
	}
	go func() {
		bufUDP := make([]byte, UDP_BUF_SIZE)
		Printf("udp server start listen video control port[%d]", s.VControlPort)
		defer Printf("udp server stop listen video control port[%d]", s.VControlPort)
		for !s.Stoped {
			if n, _, err := s.VControlConn.ReadFromUDP(bufUDP); err == nil {
				//Printf("Package recv from VControlConn.len:%d\n", n)
				s.AddInputBytes(n)
				// pack := RTPPack{
				// 	Type: RTP_TYPE_VIDEOCONTROL,
				// }
				// pack.Unmarshal(bufUDP[:n])
				// s.HandleRTP(pack)
			} else {
				Println("udp server read video control pack error", err)
				continue
			}
		}
	}()
	return
}
