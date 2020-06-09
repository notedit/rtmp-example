package main

import (
	"fmt"
	"github.com/notedit/rtmp/av"
	"github.com/notedit/rtmp/format/flv"
	"github.com/notedit/rtmp/format/rtmp"
	"github.com/notedit/rtmp/pubsub"
	"net"
	"net/http"
	"sync"
	"time"
)

type stream struct {
	videoSeq av.Packet
	audioSeq av.Packet
	conn *rtmp.Conn
}

var pub *stream
var flvmuxer *flv.Muxer

var writeVideoSeq bool
var writeAudioSeq bool



var lock sync.RWMutex
var streams = map[string]*pubsub.PubSub{}



func startRtmp() {

	lis, err := net.Listen("tcp", ":1935")
	if err != nil {
		panic(err)
	}

	s := rtmp.NewServer()

	s.LogEvent = func(c *rtmp.Conn, nc net.Conn, e int) {
		es := rtmp.EventString[e]
		fmt.Println(nc.LocalAddr(), nc.RemoteAddr(), es)
	}

	s.HandleConn = func(c *rtmp.Conn, nc net.Conn) {

		fmt.Println(c.URL.Path)

		if c.Publishing {
			pubsuber := &pubsub.PubSub{}

			lock.Lock()
			streams[c.URL.Path] = pubsuber
			lock.Unlock()

			pubsuber.SetPub(c)
			delete(streams,c.URL.Path)
		} else {

			lock.Lock()
			pubsuber := streams[c.URL.Path]
			lock.Unlock()

			if pubsuber != nil {
				pubsuber.AddSub(c.CloseNotify(),c)
			} else {
				nc.Close()
			}
		}
	}

	for {
		nc, err := lis.Accept()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		go s.HandleNetConn(nc)
	}
}



func main() {

	http.HandleFunc("/live/live", func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "video/x-flv")
		//w.Header().Set("Transfer-Encoding", "chunked")
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

		close := make(chan bool,1)
		pubsuber.AddSub(close, muxer)

	})

	go startRtmp()

	http.ListenAndServe(":8088", nil)
}
