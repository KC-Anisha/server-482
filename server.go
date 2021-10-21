package main

import (
	"strconv"
	"github.com/gorilla/mux"
	"net/http"
	"time"
	"encoding/json"
	loggly "github.com/JamesPEarly/loggly"
)

type resTime struct {
    SystemTime string
}

type loggingResponseWriter struct {
    http.ResponseWriter
    statusCode int
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
    return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
    lrw.statusCode = code
    lrw.ResponseWriter.WriteHeader(code)
}

func ServerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	sysTime := resTime{time.Now().String()}
	json.NewEncoder(w).Encode(sysTime)
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := NewLoggingResponseWriter(w)
        next.ServeHTTP(lrw, r)

		// Tag + client init for Loggly + send message
		var tag string = "server-testing"
		client := loggly.New(tag)
		client.EchoSend("info", "Method type: " + r.Method + " | Source IP address: " + r.RemoteAddr + " | Request Path: " + r.RequestURI + " | Status Code: " + strconv.Itoa(lrw.statusCode))
    })
}


func main() {
	r := mux.NewRouter()
	r.HandleFunc("/server", ServerHandler).Methods("GET")
	wrappedRouter := loggingMiddleware(r)
	http.ListenAndServe(":8080", wrappedRouter)
}
