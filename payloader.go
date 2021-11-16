package rtsp

// AACayloader payloads AAC packets
type AACPayloader struct{}

// Payload fragments an AAC packet across one or more byte arrays
func (p *AACPayloader) Payload(mtu uint16, payload []byte) [][]byte {
	var out [][]byte
	o := make([]byte, len(payload)+4)
	//AU_HEADER_LENGTH,因为单位是bit, 除以8就是auHeader的字节长度；又因为单个auheader字节长度2字节，所以再除以2就是auheader的个数。
	o[0] = 0x00 //高位
	o[1] = 0x10 //低位
	//AU_HEADER
	o[2] = (byte)((len(payload) & 0x1fe0) >> 5) //高位
	o[3] = (byte)((len(payload) & 0x1f) << 3)   //低位
	copy(o[4:], payload)
	return append(out, o)
}

type H265Payloader struct{}

func (p *H265Payloader) Payload(mtu uint16, payload []byte) [][]byte {
	return nil
}
