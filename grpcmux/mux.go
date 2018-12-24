package grpcmux

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/httprule"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

//ServeMux the custom serve mux that implement grpc ServeMux to simplify the http restful
type ServeMux struct {
	*runtime.ServeMux
}

// NewServeMux allocates and returns a new ServeMux.
func NewServeMux(opts ...runtime.ServeMuxOption) *ServeMux {
	//fix http error for grpc gateway v1.5.0
	runtime.HTTPError = defaultHTTPError
	opts = append(opts, runtime.WithOutgoingHeaderMatcher(defaultOutgoingHeaderMatcher))
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
		if key == "Authorization" || strings.HasPrefix(key, "X-") {
			md[strings.ToLower(key)] = vals
		} else {
			key = textproto.CanonicalMIMEHeaderKey(key)
			if isPermanentHTTPHeader(key) {
				md[runtime.MetadataPrefix+key] = vals
			} else if strings.HasPrefix(key, runtime.MetadataHeaderPrefix) {
				md[key[lenMetadataHeaderPrefix:]] = vals
			}
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
// permanent request headers maintained by IANA.
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

// DefaultHTTPError is the default implementation of HTTPError.
// If "err" is an error from gRPC system, the function replies with the status code mapped by HTTPStatusFromCode.
// If otherwise, it replies with http.StatusInternalServerError.
//
// The response body returned by this function is a JSON object,
// which contains a member whose key is "error" and whose value is err.Error().
func defaultHTTPError(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
	const fallback = `{"error": "failed to marshal error message"}`
	w.Header().Del("Trailer")
	w.Header().Set("Content-Type", marshaler.ContentType())

	s, ok := status.FromError(err)
	if !ok {
		s = status.New(codes.Unknown, err.Error())
	}
	code := s.Code()
	body := &errorBody{
		Error:            CodeToError(code),
		ErrorDescription: s.Message(),
		Details:          s.Proto().GetDetails(),
	}
	buf, merr := marshaler.Marshal(body)
	if merr != nil {
		grpclog.Infof("Failed to marshal error message %q: %v", body, merr)
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := io.WriteString(w, fallback); err != nil {
			grpclog.Infof("Failed to write response: %v", err)
		}
		return
	}

	md, ok := runtime.ServerMetadataFromContext(ctx)
	if !ok {
		grpclog.Infof("Failed to extract ServerMetadata from context")
	}

	handleForwardResponseServerMetadata(w, mux, md)
	handleForwardResponseTrailerHeader(w, md)
	var st int
	if st > 100 {
		st = httpStatusCode(s.Code())
	} else {
		st = runtime.HTTPStatusFromCode(s.Code())
	}
	w.WriteHeader(st)
	if _, err := w.Write(buf); err != nil {
		grpclog.Infof("Failed to write response: %v", err)
	}

	handleForwardResponseTrailer(w, md)
}

type errorBody struct {
	Error            string     `protobuf:"bytes,1,name=error" json:"error"`
	ErrorDescription string     `protobuf:"bytes,1,name=error_description" json:"error_description"`
	Details          []*any.Any `protobuf:"bytes,3,rep,name=details" json:"details,omitempty"`
}

// Make this also conform to proto.Message for builtin JSONPb Marshaler
func (e *errorBody) Reset()         { *e = errorBody{} }
func (e *errorBody) String() string { return proto.CompactTextString(e) }
func (*errorBody) ProtoMessage()    {}

var defaultOutgoingHeaderMatcher = func(key string) (string, bool) {
	return fmt.Sprintf("%s%s", runtime.MetadataHeaderPrefix, key), true
}

func handleForwardResponseServerMetadata(w http.ResponseWriter, mux *runtime.ServeMux, md runtime.ServerMetadata) {
	for k, vs := range md.HeaderMD {
		if h, ok := defaultOutgoingHeaderMatcher(k); ok {
			for _, v := range vs {
				w.Header().Add(h, v)
			}
		}
	}
}

func handleForwardResponseTrailerHeader(w http.ResponseWriter, md runtime.ServerMetadata) {
	for k := range md.TrailerMD {
		tKey := textproto.CanonicalMIMEHeaderKey(fmt.Sprintf("%s%s", runtime.MetadataTrailerPrefix, k))
		w.Header().Add("Trailer", tKey)
	}
}

func handleForwardResponseTrailer(w http.ResponseWriter, md runtime.ServerMetadata) {
	for k, vs := range md.TrailerMD {
		tKey := fmt.Sprintf("%s%s", runtime.MetadataTrailerPrefix, k)
		for _, v := range vs {
			w.Header().Add(tKey, v)
		}
	}
}

// CodesErrors some erorrs string for grpc codes
var CodesErrors = map[codes.Code]string{
	codes.OK:                 "ok",
	codes.Canceled:           "canceled",
	codes.Unknown:            "unknown",
	codes.InvalidArgument:    "invalid_argument",
	codes.DeadlineExceeded:   "deadline_exceeded",
	codes.NotFound:           "not_found",
	codes.AlreadyExists:      "already_exists",
	codes.PermissionDenied:   "permission_denied",
	codes.ResourceExhausted:  "resource_exhausted",
	codes.FailedPrecondition: "failed_precondition",
	codes.Aborted:            "aborted",
	codes.OutOfRange:         "out_of_range",
	codes.Unimplemented:      "unimplemented",
	codes.Internal:           "internal",
	codes.Unavailable:        "unavailable",
	codes.DataLoss:           "data_loss",
	codes.Unauthenticated:    "unauthenticated",
}

// CodeToError translate grpc codes to error
func CodeToError(c codes.Code) string {
	errStr, ok := CodesErrors[c]
	if ok {
		return errStr
	}
	return strconv.FormatInt(int64(c), 10)
}

// httpStatusCode the 2xxx is 200, the 4xxx is 400, the 5xxx is 500
func httpStatusCode(code codes.Code) (status int) {
	for code >= 10 {
		code = code / 10
	}
	switch code {
	case 2:
		status = 200
	case 4:
		status = 400
	default:
		status = 500
	}
	return
}
