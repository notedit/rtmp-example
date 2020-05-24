package main

import (
	"fmt"
	"github.com/notedit/gst"
	"github.com/notedit/rtmp/av"
	"github.com/notedit/rtmp/codec/h264"
	"github.com/notedit/rtmp/codec/opus"
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

const gstVideoPipeline = "videotestsrc ! video/x-raw,framerate=15/1 ! x264enc aud=false bframes=0 speed-preset=veryfast key-int-max=30 ! video/x-h264,stream-format=avc,profile=baseline ! h264parse ! appsink name=videosink "
const gstAudioPipeline = "audiotestsrc wave=sine ! audio/x-raw, rate=48000, channels=2 ! audioconvert ! opusenc ! appsink name=audiosink"

type Sample struct {
	Data     []byte
	Video    bool
	Duration uint64
}

func pullVideoSample(element *gst.Element, out chan *Sample) {

	for {

		sle, err := element.PullSample()
		if err != nil {
			if element.IsEOS() == true {
				fmt.Println("eos")
				return
			} else {
				fmt.Println(err)
				continue
			}
		}

		sample := &Sample{
			Data:     sle.Data,
			Video:    true,
			Duration: sle.Duration,
		}

		out <- sample
	}
}

func pullAudioSample(element *gst.Element, out chan *Sample) {

	for {

		sle, err := element.PullSample()
		if err != nil {
			if element.IsEOS() == true {
				fmt.Println("eos")
				return
			} else {
				fmt.Println(err)
				continue
			}
		}

		sample := &Sample{
			Data:     sle.Data,
			Video:    false,
			Duration: sle.Duration,
		}

		out <- sample
	}

}

func startPush() {

	samples := make(chan *Sample, 10)

	err := gst.CheckPlugins([]string{"x264", "videoparsersbad", "opus"})

	if err != nil {
		panic(err)
	}

	vpipeline, err := gst.ParseLaunch(gstVideoPipeline)

	if err != nil {
		panic(err)
	}

	velement := vpipeline.GetByName("videosink")
	vpipeline.SetState(gst.StatePlaying)

	go pullVideoSample(velement, samples)

	apipeline, err := gst.ParseLaunch(gstAudioPipeline)

	if err != nil {
		panic(err)
	}

	aelement := apipeline.GetByName("audiosink")
	apipeline.SetState(gst.StatePlaying)

	go pullAudioSample(aelement, samples)

	rtmpClient := rtmp.NewClient()

	var c *rtmp.Conn
	c, _, err = rtmpClient.Dial("rtmp://localhost/live/live", rtmp.PrepareWriting)

	if err != nil {
		panic(err)
	}

	h264Codec := h264.NewCodec()
	decodeConfig := av.Packet{
		Type:       av.H264DecoderConfig,
		IsKeyFrame: true,
	}

	opusCodec := &opus.Codec{
		SampleRate: 48000,
		Channels:   2,
	}

	start := time.Now()

	for sample := range samples {

		if sample.Video {
			nalus, _ := h264.SplitNALUs(sample.Data)
			nalus_ := make([][]byte, 0)
			keyframe := false
			for _, nalu := range nalus {
				typ := h264.NALUType(nalu)
				switch typ {
				case h264.NALU_SPS:
					h264Codec.AddSPSPPS(nalu)
				case h264.NALU_PPS:
					h264Codec.AddSPSPPS(nalu)
					decodeConfig.Data = make([]byte, 2048)
					var len int
					h264Codec.ToConfig(decodeConfig.Data, &len)
					decodeConfig.Data = decodeConfig.Data[0:len]
					c.WritePacket(decodeConfig)
				case h264.NALU_IDR:
					nalus_ = append(nalus_, nalu)
				case h264.NALU_NONIDR:
					nalus_ = append(nalus_, nalu)
					keyframe = true
					break
				}
			}

			duration := time.Since(start)
			data := h264.FillNALUsAVCC(nalus_)
			pkt := av.Packet{
				Type:       av.H264,
				IsKeyFrame: keyframe,
				Time:       duration,
				CTime:      duration,
				Data:       data,
			}
			c.WritePacket(pkt)

		} else {

			duration := time.Since(start)
			pkt := av.Packet{
				Type:  av.OPUS,
				Time:  duration,
				CTime: duration,
				Data:  sample.Data,
				OPUS:  opusCodec,
			}
			c.WritePacket(pkt)
		}

	}

	vpipeline.SetState(gst.StateNull)
	apipeline.SetState(gst.StateNull)

	vpipeline = nil
	velement = nil

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
				fmt.Println("server  sps  pps ========")
			case av.H264:
				fmt.Println("server h264 ", pkt.Time)
			case av.AACDecoderConfig:
				pub.audioSeq = pkt
			case av.AAC:
				fmt.Println("server aac", pkt.Time)
			case av.OPUS:
				fmt.Println("server opus", pkt.Time)
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

	go startRtmp()

	// sleep for a while
	time.Sleep(time.Second)

	startPush()
}
