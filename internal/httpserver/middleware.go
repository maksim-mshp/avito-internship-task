package httpserver

import (
	"log"
	"net/http"
	"time"
)

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.String())
		next.ServeHTTP(w, r)
		log.Printf("completed in %s", time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func Recover(next HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				RespondError(recorder, http.StatusInternalServerError, "internal error")
			}
		}()
		if err := next(recorder, r); err != nil {
			log.Printf("error: %v", err)
			RespondError(recorder, http.StatusInternalServerError, err.Error())
			return
		}
		if recorder.status >= 500 {
			log.Printf("server error %d on %s %s", recorder.status, r.Method, r.URL.String())
		}
	})
}
