package main

import (
	"bytes"
	"fmt"
	"github.com/notedit/rtmp/format/flv"
	"github.com/notedit/rtmp/pubsub"
	"log"
	"net/http"
	"sync"
)

var lock sync.RWMutex
var streams = map[string]*pubsub.PubSub{}

func pushRequest(url string) *bytes.Buffer {

	client := http.Client{}

	var buf bytes.Buffer
	req, err := http.NewRequest("POST", url, &buf)
	req.Header.Set("Content-Type", "application/octet-stream")

	if err != nil {
		log.Fatalln(err)
	}

	go func() {
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalln(err)
		}
		log.Println(resp)
	}()

	return &buf
}

func startHttpServer() {

	http.HandleFunc("/live/live", func(w http.ResponseWriter, r *http.Request) {

		if r.Method == "POST" {

			fmt.Println("POST ", r.RequestURI)

			// HTTP POST
			demuxer := flv.NewDemuxer(r.Body)
			pubsuber := &pubsub.PubSub{}
			lock.Lock()
			streams["/live/live"] = pubsuber
			lock.Unlock()
			pubsuber.SetPub(demuxer)
			delete(streams,"/live/live")


			fmt.Println("POST ", "============")


		} else if r.Method == "GET" {

			fmt.Println("GET ", r.RequestURI)

			// HTTP GET
			w.Header().Set("Content-Type", "video/x-flv")
			w.Header().Set("Transfer-Encoding", "chunked")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(200)

			flusher := w.(http.Flusher)
			flusher.Flush()

			lock.Lock()
			pubsuber := streams["/live/live"]
			lock.Unlock()

			if pubsuber == nil {
				return
			}

			muxer := flv.NewMuxer(w)
			muxer.HasAudio = true
			muxer.HasVideo = true
			muxer.Publishing = true

			muxer.WriteFileHeader()

			close := make(chan bool, 1)

			fmt.Println("addsub")
			pubsuber.AddSub(close, muxer)

		}

	})

	http.ListenAndServe(":8088", nil)
}

func main() {

	startHttpServer()

}
