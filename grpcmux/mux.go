package grpcmux

import (
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/httprule"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"net"
	"net/http"
	"strings"
)

//ServeMux the custom serve mux that implement grpc ServeMux to simplify the http restful
type ServeMux struct {
	*runtime.ServeMux
}

// NewServeMux allocates and returns a new ServeMux.
func NewServeMux(opts ...runtime.ServeMuxOption) *ServeMux {
	return &ServeMux{runtime.NewServeMux(opts...)}
}

//Handle associates "h" to the pair of HTTP method and path pattern.
func (s *ServeMux) Handle(method string, path string, h runtime.HandlerFunc) {
	pattern := runtime.MustPattern(parsePatternURL(path))
	s.ServeMux.Handle(method, pattern, h)
}

//ServeHTTP add ctx from http request
func (s *ServeMux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	md := annotateMetadata(req)
	ctx := metadata.NewIncomingContext(req.Context(), md)
	req = req.WithContext(ctx)
	s.ServeMux.ServeHTTP(w, req.WithContext(ctx))
}

var lenMetadataHeaderPrefix = len(runtime.MetadataHeaderPrefix)

const xForwardedFor = "x-forwarded-for"
const xForwardedHost = "x-forwarded-host"

//annotateMetadata
func annotateMetadata(req *http.Request) metadata.MD {
	md := make(metadata.MD)
	for key, vals := range req.Header {
		lowerKey := strings.ToLower(key)
		if isPermanentHTTPHeader(key) {
			md[runtime.MetadataPrefix+lowerKey] = vals
		} else if strings.HasPrefix(key, runtime.MetadataHeaderPrefix) {
			md[lowerKey[lenMetadataHeaderPrefix:]] = vals
		} else {
			md[lowerKey] = vals
		}
	}
	if len(md[xForwardedHost]) == 0 && req.Host != "" {
		md[xForwardedHost] = []string{req.Host}
	}
	if addr := req.RemoteAddr; addr != "" {
		if remoteIP, _, err := net.SplitHostPort(addr); err == nil {
			if len(md[xForwardedFor]) == 0 {
				md[xForwardedFor] = []string{remoteIP}
			} else {
				md[xForwardedFor] = []string{fmt.Sprintf("%s, %s", md[xForwardedFor][0], remoteIP)}
			}
		} else {
			grpclog.Infof("invalid remote addr: %s", addr)
		}
	}
	return md
}

func parsePatternURL(path string) (runtime.Pattern, error) {
	compiler, err := httprule.Parse(path)
	if err != nil {
		return runtime.Pattern{}, err
	}
	tp := compiler.Compile()
	return runtime.NewPattern(tp.Version, tp.OpCodes, tp.Pool, tp.Verb)
}

// isPermanentHTTPHeader checks whether hdr belongs to the list of
// permenant request headers maintained by IANA.
// http://www.iana.org/assignments/message-headers/message-headers.xml
func isPermanentHTTPHeader(hdr string) bool {
	switch hdr {
	case
		"Accept",
		"Accept-Charset",
		"Accept-Language",
		"Accept-Ranges",
		"Authorization",
		"Cache-Control",
		"Content-Type",
		"Cookie",
		"Date",
		"Expect",
		"From",
		"Host",
		"If-Match",
		"If-Modified-Since",
		"If-None-Match",
		"If-Schedule-Tag-Match",
		"If-Unmodified-Since",
		"Max-Forwards",
		"Origin",
		"Pragma",
		"Referer",
		"User-Agent",
		"Via",
		"Warning":
		return true
	}
	return false
}
