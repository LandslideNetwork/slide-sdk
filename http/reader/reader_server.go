package reader

import (
	"context"
	"io"

	readerpb "github.com/consideritdone/landslidevm/proto/io/reader"
)

var _ readerpb.ReaderServer = (*Server)(nil)

// Server is an io.Reader that is managed over RPC.
type Server struct {
	readerpb.UnsafeReaderServer
	reader io.Reader
}

// NewServer returns an io.Reader instance managed remotely
func NewServer(reader io.Reader) *Server {
	return &Server{reader: reader}
}

func (s *Server) Read(_ context.Context, req *readerpb.ReadRequest) (*readerpb.ReadResponse, error) {
	buf := make([]byte, int(req.Length))
	n, err := s.reader.Read(buf)
	resp := &readerpb.ReadResponse{
		Read: buf[:n],
	}
	if err != nil {
		errStr := err.Error()
		resp.Error = &errStr
	}
	return resp, nil
}
