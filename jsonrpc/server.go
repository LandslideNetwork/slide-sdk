package jsonrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

var (
	_ Handler      = (HandlerFunc)(nil)
	_ http.Handler = (*rpcServer)(nil)
	_ Server       = (*rpcServer)(nil)
)

type (
	Server interface {
		http.Handler

		Handle(string, Handler)
		HandleFunc(string, HandlerFunc)
	}

	HandlerFunc func(context.Context, json.RawMessage) (json.RawMessage, error)

	Handler interface {
		Dispatch(context.Context, json.RawMessage) (json.RawMessage, error)
	}

	rpcServer struct {
		mu       *sync.Mutex
		handlers map[string]Handler
	}
)

func (f HandlerFunc) Dispatch(ctx context.Context, data json.RawMessage) (json.RawMessage, error) {
	return f(ctx, data)
}

func NewServer(handlers map[string]Handler) Server {
	return &rpcServer{mu: new(sync.Mutex), handlers: handlers}
}

func (s *rpcServer) Handle(name string, handler Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[name] = handler
}

func (s rpcServer) HandleFunc(name string, handler HandlerFunc) {
	s.Handle(name, handler)
}

func (s rpcServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		req Request
		res Response
	)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		res = Response{
			single: true,
			params: []ResponseParams{{Error: &ErrParseError}},
		}
	} else {
		defer r.Body.Close()

		res = Response{
			single: req.single,
			params: make([]ResponseParams, len(req.params)),
		}
		for i := range req.params {
			res.params[i].ID = req.params[i].ID
			if req.params[i].JsonRPC != "2.0" {
				res.params[i].Error = &ErrInvalidRequest
			} else if handler, ok := s.handlers[req.params[i].Method]; !ok {
				res.params[i].Error = &ErrInvalidMethod
			} else if data, err := handler.Dispatch(r.Context(), req.params[i].Params); err != nil {
				e := ErrInternalError
				e.Data = err
				res.params[i].Error = &e
			} else {
				res.params[i].Result = data
			}
		}
	}

	json.NewEncoder(w).Encode(res)
}
