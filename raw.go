package ipmi

const MasterMaxSize = 16

//
type MasterRequest struct {
	Bus   byte
	Addr  byte
	Data  []byte
	Rsize byte
}

//
type MasterResponse struct {
	CompletionCode
	Data []byte
}

func (r *MasterRequest) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 3+len(r.Data))
	buf[0] = r.Bus
	buf[1] = r.Addr
	buf[2] = r.Rsize
	copy(buf[3:], r.Data)
	return buf, nil
}

func (r *MasterRequest) UnmarshalBinary(buf []byte) error {
	if len(buf) < 3 {
		return ErrShortPacket
	}
	r.Bus = buf[0]
	r.Addr = buf[1]
	r.Rsize = buf[2]
	r.Data = buf[3:]
	return nil
}

func (r *MasterResponse) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+len(r.Data))
	buf[0] = byte(r.CompletionCode)
	buf[1] = byte(len(r.Data))
	copy(buf[2:], r.Data)
	return buf, nil
}

func (r *MasterResponse) UnmarshalBinary(buf []byte) error {
	if len(buf) < 2 {
		return ErrShortPacket
	}
	r.CompletionCode = CompletionCode(buf[0])
	r.Data = buf[1:]
	return nil
}
