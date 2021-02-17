package rtsp

import (
	"fmt"
	"net"
	"strings"

	. "github.com/Monibuca/utils/v3"
)

type UDPClient struct {
	APort        int
	AConn        *net.UDPConn
	AControlPort int
	AControlConn *net.UDPConn
	VPort        int
	VConn        *net.UDPConn
	VControlPort int
	VControlConn *net.UDPConn

	Stoped bool
}

func (s *UDPClient) Stop() {
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

func (c *UDPClient) SetupAudio() (err error) {
	defer func() {
		if err != nil {
			Println(err)
			c.Stop()
		}
	}()
	host := c.AConn.RemoteAddr().String()
	host = host[:strings.LastIndex(host, ":")]
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, c.APort))
	if err != nil {
		return
	}
	c.AConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	networkBuffer := 1048576
	if err := c.AConn.SetReadBuffer(networkBuffer); err != nil {
		Printf("udp client audio conn set read buffer error, %v", err)
	}
	if err := c.AConn.SetWriteBuffer(networkBuffer); err != nil {
		Printf("udp client audio conn set write buffer error, %v", err)
	}

	addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, c.AControlPort))
	if err != nil {
		return
	}
	c.AControlConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	if err := c.AControlConn.SetReadBuffer(networkBuffer); err != nil {
		Printf("udp client audio control conn set read buffer error, %v", err)
	}
	if err := c.AControlConn.SetWriteBuffer(networkBuffer); err != nil {
		Printf("udp client audio control conn set write buffer error, %v", err)
	}
	return
}

func (c *UDPClient) SetupVideo() (err error) {
	defer func() {
		if err != nil {
			Println(err)
			c.Stop()
		}
	}()
	host := c.VConn.RemoteAddr().String()
	host = host[:strings.LastIndex(host, ":")]
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, c.VPort))
	if err != nil {
		return
	}
	c.VConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	networkBuffer := 1048576
	if err := c.VConn.SetReadBuffer(networkBuffer); err != nil {
		Printf("udp client video conn set read buffer error, %v", err)
	}
	if err := c.VConn.SetWriteBuffer(networkBuffer); err != nil {
		Printf("udp client video conn set write buffer error, %v", err)
	}

	addr, err = net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", host, c.VControlPort))
	if err != nil {
		return
	}
	c.VControlConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return
	}
	if err := c.VControlConn.SetReadBuffer(networkBuffer); err != nil {
		Printf("udp client video control conn set read buffer error, %v", err)
	}
	if err := c.VControlConn.SetWriteBuffer(networkBuffer); err != nil {
		Printf("udp client video control conn set write buffer error, %v", err)
	}
	return
}

func (c *UDPClient) SendRTP(pack *RTPPack) (err error) {
	if pack == nil {
		err = fmt.Errorf("udp client send rtp got nil pack")
		return
	}
	var conn *net.UDPConn
	switch pack.Type {
	case RTP_TYPE_AUDIO:
		conn = c.AConn
	case RTP_TYPE_AUDIOCONTROL:
		conn = c.AControlConn
	case RTP_TYPE_VIDEO:
		conn = c.VConn
	case RTP_TYPE_VIDEOCONTROL:
		conn = c.VControlConn
	default:
		err = fmt.Errorf("udp client send rtp got unkown pack type[%v]", pack.Type)
		return
	}
	if conn == nil {
		err = fmt.Errorf("udp client send rtp pack type[%v] failed, conn not found", pack.Type)
		return
	}

	if _, err = conn.Write(pack.Raw); err != nil {
		err = fmt.Errorf("udp client write bytes error, %v", err)
		return
	}
	// Printf("udp client write [%d/%d]", n, pack.Buffer.Len())
	return
}
