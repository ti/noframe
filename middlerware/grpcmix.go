package middlerware

import (
	"net/http"
	"strings"
)

const grpcContentType = "application/grpc"

//GRPCMixHandler mix grpc and http in one Handler
func GRPCMixHandler(h http.Handler, grpc http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpc.ServeHTTP(w, r)
		} else {
			h.ServeHTTP(w, r)
		}
	})
}
