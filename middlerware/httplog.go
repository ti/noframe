package middlerware

import (
	"bufio"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
)

type loggingHandler struct {
	handler http.Handler
}

func (h loggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := makeLogger(w)
	url := *req.URL
	h.handler.ServeHTTP(logger, req)
	log.WithFields(log.Fields{
		"module": "access",
		"method": req.Method,
		"url":    url.Path,
		"status": logger.Status(),
		"size":   logger.Size(),
	}).Info()
}

func makeLogger(w http.ResponseWriter) loggingResponseWriter {
	var logger loggingResponseWriter = &responseLogger{w: w, status: http.StatusOK}
	if _, ok := w.(http.Hijacker); ok {
		logger = &hijackLogger{responseLogger{w: w, status: http.StatusOK}}
	}
	h, ok1 := logger.(http.Hijacker)
	c, ok2 := w.(http.CloseNotifier)
	if ok1 && ok2 {
		return hijackCloseNotifier{logger, h, c}
	}
	if ok2 {
		return &closeNotifyWriter{logger, c}
	}
	return logger
}

type responseLogger struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (l *responseLogger) Header() http.Header {
	return l.w.Header()
}

func (l *responseLogger) Write(b []byte) (int, error) {
	size, err := l.w.Write(b)
	l.size += size
	return size, err
}

func (l *responseLogger) WriteHeader(s int) {
	l.w.WriteHeader(s)
	l.status = s
}

func (l *responseLogger) Status() int {
	return l.status
}

func (l *responseLogger) Size() int {
	return l.size
}

func (l *responseLogger) Flush() {
	f, ok := l.w.(http.Flusher)
	if ok {
		f.Flush()
	}
}

type hijackLogger struct {
	responseLogger
}

func (l *hijackLogger) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h := l.responseLogger.w.(http.Hijacker)
	conn, rw, err := h.Hijack()
	if err == nil && l.responseLogger.status == 0 {
		l.responseLogger.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

type closeNotifyWriter struct {
	loggingResponseWriter
	http.CloseNotifier
}

type hijackCloseNotifier struct {
	loggingResponseWriter
	http.Hijacker
	http.CloseNotifier
}

//LoggingHandler write log over http
func LoggingHandler(h http.Handler) http.Handler {
	return loggingHandler{h}
}

type loggingResponseWriter interface {
	http.ResponseWriter
	http.Flusher
	Status() int
	Size() int
}
