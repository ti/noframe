package middlerware

import (
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"net/http"
	"strings"
)

//GRPCMixOptions the Options of GRPC Mix
type GRPCMixOptions struct {
	server *http2.Server
}

//GRPCMixOption the Option of GRPC Mix
type GRPCMixOption func(*GRPCMixOptions)

func GRPCMixWithServer(s *http2.Server) GRPCMixOption {
	return func(o *GRPCMixOptions) {
		o.server = s
	}
}

//GRPCMixHandler mix grpc and http in one Handler
func GRPCMixHandler(h http.Handler, grpc *grpc.Server, opts ...GRPCMixOption) http.Handler {
	mix := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpc.ServeHTTP(w, r)
		} else {
			h.ServeHTTP(w, r)
		}
	})
	var options GRPCMixOptions
	for _, o := range opts {
		o(&options)
	}
	server := options.server
	if server == nil {
		server = &http2.Server{}
	}
	return h2c.NewHandler(mix, server)
}
