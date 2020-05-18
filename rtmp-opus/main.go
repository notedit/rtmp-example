package main

import (
	"fmt"
	"github.com/notedit/gst"
	"github.com/notedit/rtmp/av"
	"github.com/notedit/rtmp/codec/h264"
	"github.com/notedit/rtmp/format/rtmp"
	"net"
	"time"
)

type stream struct {
	videoSeq av.Packet
	audioSeq av.Packet
	conn     *rtmp.Conn
}

var pub *stream

const gstPipeline = "videotestsrc ! video/x-raw,framerate=15/1 ! x264enc aud=false bframes=0 speed-preset=veryfast key-int-max=15 ! video/x-h264,stream-format=avc,profile=baseline ! h264parse ! appsink name=videosink "

func startPush() {

	err := gst.CheckPlugins([]string{"x264", "videoparsersbad"})

	if err != nil {
		panic(err)
	}

	pipeline, err := gst.ParseLaunch(gstPipeline)

	if err != nil {
		panic(err)
	}

	element := pipeline.GetByName("videosink")
	pipeline.SetState(gst.StatePlaying)

	rtmpClient := rtmp.NewClient()

	var c *rtmp.Conn
	var nc net.Conn
	c, nc, err = rtmpClient.Dial("rtmp://localhost/live/live", rtmp.PrepareWriting)

	if err != nil {
		panic(err)
	}

	h264Codec := h264.NewCodec()

	for {
		sample, err := element.PullSample()
		if err != nil {
			if element.IsEOS() == true {
				fmt.Println("eos")
				return
			} else {
				fmt.Println(err)
				continue
			}
		}

		nalus, _ := h264.SplitNALUs(sample.Data)

		for _, nalu := range nalus {
			typ := h264.NALUType(nalu)
			switch typ {
			case h264.NALU_SPS:
				h264Codec.AddSPSPPS(nalu)
			case h264.NALU_PPS:
				h264Codec.AddSPSPPS(nalu)
			case h264.NALU_IDR:

			case h264.NALU_NONIDR:
				break
			}
			fmt.Println(h264.NALUTypeString(h264.NALUType(nalu)))
		}
		fmt.Println("got sample", sample.Duration, len(nalus), typ)
	}

	pipeline.SetState(gst.StateNull)

	pipeline = nil
	element = nil
}

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
				fmt.Println("Write video ", pkt.Time)
			case av.AACDecoderConfig:
				pub.audioSeq = pkt
			case av.AAC:
				fmt.Println("Write audio ", pkt.Time)
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

	startPush()
}
