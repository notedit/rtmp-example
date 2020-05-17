package main

import (
	"fmt"
	"github.com/notedit/rtmp/av"
	"github.com/notedit/rtmp/format/flv"
	"github.com/notedit/rtmp/format/rtmp"
	"net"
	"net/http"
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

		if !c.Publishing {
			return
		}

		pub := &stream{}
		pub.conn = c

		for {
			pkt, err := c.ReadPacket()
			if err != nil {
				return
			}
			switch pkt.Type {
			case av.H264DecoderConfig:
				pub.videoSeq = pkt
			case av.H264:
				if !writeVideoSeq {
					if pkt.IsKeyFrame && flvmuxer !=nil {
						writeVideoSeq = true
						flvmuxer.WritePacket(pub.videoSeq)
						flvmuxer.WritePacket(pkt)
					} else {
						continue
					}
				} else {
					flvmuxer.WritePacket(pkt)
					fmt.Println("Write video ")
				}
			case av.AACDecoderConfig:
				pub.audioSeq = pkt
			case av.AAC:
				if !writeAudioSeq {
					if flvmuxer !=nil {
						writeAudioSeq = true
						flvmuxer.WritePacket(pub.audioSeq)
					}
				} else {
					flvmuxer.WritePacket(pkt)
					fmt.Println("Write audio ")
				}
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

	http.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "video/x-flv")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(200)

		flusher := w.(http.Flusher)
		flusher.Flush()

		muxer := flv.NewMuxer(w)
		muxer.HasAudio = true
		muxer.HasVideo = true
		muxer.Publishing = true

		muxer.WriteFileHeader()
		flvmuxer = muxer

		done := r.Context().Done()
		<-done
	})

	go startRtmp()

	http.ListenAndServe(":8088", nil)
}
