package grpcmux

import (
	"context"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"io"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

//ServeMux the custom serve mux that implement grpc ServeMux to simplify the http restful
type ServeMux struct {
	*runtime.ServeMux
}

// DefaultHeaderMatcher default header matcher
func DefaultHeaderMatcher(key string) (string, bool) {
	return strings.ToLower(key), true
}

// NewServeMux allocates and returns a new ServeMux.
func NewServeMux(opts ...runtime.ServeMuxOption) *ServeMux {
	if len(opts) == 0 {
		opts = append(opts,
			runtime.WithIncomingHeaderMatcher(DefaultHeaderMatcher),
			runtime.WithErrorHandler(DefaultHTTPErrorHandler),
			runtime.WithMarshalerOption("*", defaultMarshaler),
		)
	}

	return &ServeMux{runtime.NewServeMux(opts...)}
}

// Handle associates "h" to the pair of HTTP method and path pattern.
func (s *ServeMux) Handle(method string, path string, h runtime.HandlerFunc) {
	err := s.ServeMux.HandlePath(method, path, h)
	if err != nil {
		panic(err)
	}
}

//MuxedGrpc check the context is by mux grpc
type MuxedGrpc struct {
}

//ServeHTTP add ctx from http request
func (s *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Header.Set("x-request-path", r.URL.Path)
	r.Header.Set("x-request-method", r.Method)
	s.ServeMux.ServeHTTP(w, r)
}

const fallback = `{"error": "internal", "error_description":"failed to marshal error message"}`

// DefaultHTTPErrorHandler is the default error handler.
// If "err" is a gRPC Status, the function replies with the status code mapped by HTTPStatusFromCode.
// If otherwise, it replies with http.StatusInternalServerError.
//
// The response body written by this function is a Status message marshaled by the Marshaler.
func DefaultHTTPErrorHandler(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, r *http.Request, err error) {
	s := status.Convert(err)
	pb := s.Proto()

	w.Header().Del("Trailer")
	w.Header().Del("Transfer-Encoding")

	contentType := marshaler.ContentType(pb)
	w.Header().Set("Content-Type", contentType)

	body := &Error{
		Error:            CodeToError(s.Code()),
		ErrorDescription: s.Message(),
		ErrorCode:        int32(s.Code()),
		Details:          s.Proto().GetDetails(),
	}
	buf, merr := marshaler.Marshal(body)
	if merr != nil {
		grpclog.Infof("Failed to marshal error message %q: %v", s, merr)
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

	// RFC 7230 https://tools.ietf.org/html/rfc7230#section-4.1.2
	// Unless the request includes a TE header field indicating "trailers"
	// is acceptable, as described in Section 4.3, a server SHOULD NOT
	// generate trailer fields that it believes are necessary for the user
	// agent to receive.
	var wantsTrailers bool

	if te := r.Header.Get("TE"); strings.Contains(strings.ToLower(te), "trailers") {
		wantsTrailers = true
		handleForwardResponseTrailerHeader(w, md)
		w.Header().Set("Transfer-Encoding", "chunked")
	}

	st := int(s.Code())
	if st > 100 {
		st = httpStatusCode(s.Code())
	} else {
		st = runtime.HTTPStatusFromCode(s.Code())
	}
	w.WriteHeader(st)
	if _, err := w.Write(buf); err != nil {
		grpclog.Infof("Failed to write response: %v", err)
	}

	if wantsTrailers {
		handleForwardResponseTrailer(w, md)
	}
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

var defaultMarshaler = &runtime.HTTPBodyMarshaler{
	Marshaler: &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			Multiline:         false,
			Indent:            "",
			AllowPartial:      false,
			UseProtoNames:     true,
			UseEnumNumbers:    false,
			EmitUnpopulated:   false,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	},
}


// WriteHTTPErrorResponse  set HTTP status code and write error description to the body.
func WriteHTTPErrorResponse(w http.ResponseWriter, err error) {
	s, ok := status.FromError(err)
	if !ok {
		s = status.New(codes.Unknown, err.Error())
	}
	code := s.Code()
	body := &Error{
		Error:            CodeToError(s.Code()),
		ErrorDescription: s.Message(),
		ErrorCode:        int32(s.Code()),
		Details:          s.Proto().GetDetails(),
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	buf, merr := defaultMarshaler.Marshal(body)
	if merr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := io.WriteString(w, fallback); err != nil {
			grpclog.Infof("Failed to write response: %v", err)
		}
		return
	}
	st := int(code)
	if st > 100 {
		st = httpStatusCode(code)
	} else {
		st = runtime.HTTPStatusFromCode(code)
	}
	w.WriteHeader(st)
	w.Write(buf)
}
