package grpcmux

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
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
	runtime.HTTPError = DefaultHTTPError
	opts = append(opts, runtime.WithOutgoingHeaderMatcher(defaultOutgoingHeaderMatcher))
	return &ServeMux{runtime.NewServeMux(opts...)}
}

//Handle associates "h" to the pair of HTTP method and path pattern.
func (s *ServeMux) Handle(method string, path string, h runtime.HandlerFunc) {
	pattern := runtime.MustPattern(parsePatternURL(path))
	s.ServeMux.Handle(method, pattern, h)
}

//MuxedGrpc check the context is by mux grpc
type MuxedGrpc struct {
}

//ServeHTTP add ctx from http request
func (s *ServeMux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if m := req.Header.Get("X-Http-Method-Override"); m != "" {
		req.Method = m
		delete(req.Header, "X-Http-Method-Override")
	}
	timeout := runtime.DefaultContextTimeout
	cancelCtx, cancel := context.WithCancel(req.Context())
	defer cancel()
	md := annotateMetadata(req)
	rctx := metadata.NewIncomingContext(cancelCtx, md)
	rctx = context.WithValue(rctx, MuxedGrpc{}, true)
	if timeout != 0 {
		rctx, _ = context.WithTimeout(rctx, timeout)
	}
	s.ServeMux.ServeHTTP(w, req.WithContext(rctx))
}

const xForwardedFor = "x-forwarded-for"
const xForwardedHost = "x-forwarded-host"

//annotateMetadata
func annotateMetadata(req *http.Request) metadata.MD {
	md := make(metadata.MD)
	for key, vals := range req.Header {
		md[strings.ToLower(key)] = vals
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
	md["x-request-path"] = []string{req.URL.Path}
	md["x-request-method"] = []string{req.Method}
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

// DefaultHTTPError is the default implementation of HTTPError.
// If "err" is an error from gRPC system, the function replies with the status code mapped by HTTPStatusFromCode.
// If otherwise, it replies with http.StatusInternalServerError.
//
// The response body returned by this function is a JSON object,
// which contains a member whose key is "error" and whose value is err.Error().
func DefaultHTTPError(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
	const fallback = `{"error": "mux_marshal_error","error_description": "failed to marshal error message"}`
	w.Header().Del("Trailer")
	w.Header().Set("Content-Type", marshaler.ContentType())

	s, ok := status.FromError(err)
	if !ok {
		if err == context.DeadlineExceeded {
			s = status.New(codes.Canceled, "server "+err.Error())
		} else if err == context.Canceled {
			s = status.New(codes.Canceled, "client "+err.Error())
		} else {
			s = status.New(codes.Unknown, err.Error())
		}
	}
	code := s.Code()

	var details []interface{}
	for _, v := range s.Proto().GetDetails() {
		var d ptypes.DynamicAny
		err := ptypes.UnmarshalAny(v, &d)
		if err != nil {
			details = append(details, v)
		} else {
			details = append(details, d.Message)
		}
	}

	body := &errorBody{
		Error:            CodeToError(code),
		ErrorDescription: s.Message(),
		Details:          details,
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
	st := int(code)
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

// SetCustomErrorCodes set custom error codes for DefaultHTTPError
// the map[int32]string is compact to protobuf's ENMU_name
// 2*** HTTP status 200
// 4*** HTTP status 400
// 5*** AND other HTTP status 500
// For exp:
// in proto
// enum CommonError {
//	captcha_required = 4001;
//	invalid_captcha = 4002;
// }
// in code
// grpcmux.SetCustomErrorCodes(common.CommonError_name)
func SetCustomErrorCodes(codeErrors map[int32]string) {
	for code, errorMsg := range codeErrors {
		codesErrors[codes.Code(code)] = errorMsg
	}
}

// This is to make the error more compatible with users that expect errors to be Status objects:
// https://github.com/grpc/grpc/blob/master/src/proto/grpc/status/status.proto
// AND
// https://tools.ietf.org/html/rfc6749#section-5.2
// It should be the exact same error_description as the Error field.
type errorBody struct {
	Error            string        `protobuf:"bytes,1,name=error" json:"error"`
	ErrorDescription string        `protobuf:"bytes,1,name=error_description" json:"error_description,omitempty"`
	Details          []interface{} `protobuf:"bytes,1,name=details" json:"details,omitempty"`
}

// Make this also conform to proto.Message for builtin JSONPb Marshaler
func (e *errorBody) Reset()         { *e = errorBody{} }
func (e *errorBody) String() string { return proto.CompactTextString(e) }
func (*errorBody) ProtoMessage()    {}

var defaultOutgoingHeaderMatcher = func(key string) (string, bool) {
	return fmt.Sprintf("%s%s", runtime.MetadataHeaderPrefix, key), true
}

func handleForwardResponseServerMetadata(w http.ResponseWriter, _ *runtime.ServeMux, md runtime.ServerMetadata) {
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

// codesErrors some errors string for grpc codes
var codesErrors = map[codes.Code]string{
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
	errStr, ok := codesErrors[c]
	if ok {
		return errStr
	}
	return strconv.FormatInt(int64(c), 10)
}

// httpStatusCode the 2xxx is 200, the 4xxx is 400, the 5xxx is 500
func httpStatusCode(code codes.Code) (status int) {
	// http status codes can be error codes
	if code >= 200 && code < 599 {
		return int(code)
	}
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
