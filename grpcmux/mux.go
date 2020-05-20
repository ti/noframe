package grpcmux

import (
	"context"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/httprule"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
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
		opts = append(opts, runtime.WithIncomingHeaderMatcher(DefaultHeaderMatcher), runtime.WithProtoErrorHandler(DefaultHTTPError))
	}

	return &ServeMux{runtime.NewServeMux(opts...)}
}

// Handle associates "h" to the pair of HTTP method and path pattern.
func (s *ServeMux) Handle(method string, path string, h runtime.HandlerFunc) {
	pattern := runtime.MustPattern(parsePatternURL(path))
	s.ServeMux.Handle(method, pattern, h)
}

//MuxedGrpc check the context is by mux grpc
type MuxedGrpc struct {
}

//ServeHTTP add ctx from http request
func (s *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m := r.Header.Get("X-Http-Method-Override"); m != "" {
		r.Method = m
		delete(r.Header, "X-Http-Method-Override")
	}
	r.Header.Set("x-request-path", r.URL.Path)
	r.Header.Set("x-request-method", r.Method)
	s.ServeMux.ServeHTTP(w, r)
}

const fallback = `{"error": "internal", "error_description":"failed to marshal error message"}`

// DefaultHTTPError is the default implementation of HTTPError.
// If "err" is an error from gRPC system, the function replies with the status code mapped by HTTPStatusFromCode.
// If otherwise, it replies with http.StatusInternalServerError.
//
// The response body returned by this function is a JSON object,
// which contains a member whose key is "error" and whose value is err.Error().
func DefaultHTTPError(ctx context.Context, mux *runtime.ServeMux, marshaler runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
	s, ok := status.FromError(err)
	if !ok {
		s = status.New(codes.Unknown, err.Error())
	}
	w.Header().Del("Trailer")

	contentType := marshaler.ContentType()
	// Check marshaler on run time in order to keep backwards compatability
	// An interface param needs to be added to the ContentType() function on
	// the Marshal interface to be able to remove this check
	if typeMarshaler, ok := marshaler.(contentTypeMarshaler); ok {
		pb := s.Proto()
		contentType = typeMarshaler.ContentTypeFromMessage(pb)
	}

	w.Header().Set("Content-Type", contentType)

	body := &Error{
		Error:            CodeToError(s.Code()),
		ErrorDescription: s.Message(),
		ErrorCode:        int32(s.Code()),
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
	handleForwardResponseTrailer(w, md)
}

func parsePatternURL(path string) (runtime.Pattern, error) {
	compiler, err := httprule.Parse(path)
	if err != nil {
		return runtime.Pattern{}, err
	}
	tp := compiler.Compile()
	return runtime.NewPattern(tp.Version, tp.OpCodes, tp.Pool, tp.Verb)
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

// Marshalers that implement contentTypeMarshaler will have their ContentTypeFromMessage method called
// to set the Content-Type header on the response
type contentTypeMarshaler interface {
	// ContentTypeFromMessage returns the Content-Type this marshaler produces from the provided message
	ContentTypeFromMessage(v interface{}) string
}

var jsonMarshaler = &runtime.HTTPBodyMarshaler{
	Marshaler: &runtime.JSONPb{OrigName: true},
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
	buf, merr := jsonMarshaler.Marshal(body)
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
