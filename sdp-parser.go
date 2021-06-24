package rtsp

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
)

type SDPInfo struct {
	AVType             string
	Codec              string
	TimeScale          int
	Control            string
	Rtpmap             int
	Config             []byte
	SpropParameterSets [][]byte
	VPS                []byte
	PPS                []byte
	SPS                []byte
	PayloadType        int
	SizeLength         int
	IndexLength        int
}

func ParseSDP(sdpRaw string) map[string]*SDPInfo {
	sdpMap := make(map[string]*SDPInfo)
	var info *SDPInfo
	for _, line := range strings.Split(sdpRaw, "\n") {
		line = strings.TrimSpace(line)
		typeval := strings.SplitN(line, "=", 2)
		if len(typeval) == 2 {
			fields := strings.SplitN(typeval[1], " ", 2)
			switch typeval[0] {
			case "m":
				if len(fields) > 0 {
					info = &SDPInfo{AVType: fields[0]}
					sdpMap[info.AVType] = info
					mfields := strings.Split(fields[1], " ")
					if len(mfields) >= 3 {
						info.PayloadType, _ = strconv.Atoi(mfields[2])
					}
				}
			case "a":
				if info != nil {
					for _, field := range fields {
						keyval := strings.SplitN(field, ":", 2)
						if len(keyval) >= 2 {
							key := keyval[0]
							val := keyval[1]
							switch key {
							case "control":
								info.Control = val
							case "rtpmap":
								info.Rtpmap, _ = strconv.Atoi(val)
							}
						}
						keyval = strings.Split(field, "/")
						if len(keyval) >= 2 {
							switch keyval[0] {
							case "H264", "H265", "PCMA", "PCMU":
								info.Codec = keyval[0]
							case "MPEG4-GENERIC":
								info.Codec = "AAC"
							}
							if i, err := strconv.Atoi(keyval[1]); err == nil {
								info.TimeScale = i
							}
						}
						keyval = strings.Split(field, ";")
						if len(keyval) > 1 {
							for _, field := range keyval {
								keyval := strings.SplitN(field, "=", 2)
								if len(keyval) == 2 {
									key := strings.TrimSpace(keyval[0])
									val := keyval[1]
									switch key {
									case "config":
										info.Config, _ = hex.DecodeString(val)
									case "sizelength":
										info.SizeLength, _ = strconv.Atoi(val)
									case "indexlength":
										info.IndexLength, _ = strconv.Atoi(val)
									case "sprop-vps":
										info.VPS, _ = base64.StdEncoding.DecodeString(val)
									case "sprop-sps":
										info.SPS, _ = base64.StdEncoding.DecodeString(val)
									case "sprop-pps":
										info.PPS, _ = base64.StdEncoding.DecodeString(val)
									case "sprop-parameter-sets":
										fields := strings.Split(val, ",")
										for _, field := range fields {
											val, _ := base64.StdEncoding.DecodeString(field)
											info.SpropParameterSets = append(info.SpropParameterSets, val)
										}
									}
								}
							}
						}
					}
				}

			}
		}
	}
	return sdpMap
}
