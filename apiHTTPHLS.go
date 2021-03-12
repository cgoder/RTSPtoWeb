package main

import (
	"bytes"
	"context"
	"time"

	"github.com/cgoder/vdk/format/ts"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

//HTTPAPIServerStreamHLSTS send client m3u8 play list
func HTTPAPIServerStreamHLSM3U8(c *gin.Context) {
	streamID := c.Param("uuid")
	channelID := c.Param("channel")

	if !Storage.StreamChannelExist(streamID, channelID) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_hls",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamHLSM3U8",
			"call":    "StreamChannelExist",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}

	c.Header("Content-Type", "application/x-mpegURL")
	Storage.StreamChannelRun(context.Background(), streamID, channelID)

	//If stream mode on_demand need wait ready segment's
	for i := 0; i < 40; i++ {
		index, seq, err := Storage.StreamHLSm3u8(streamID, channelID)
		if err != nil {
			c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
			log.WithFields(logrus.Fields{
				"module":  "http_hls",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamHLSM3U8",
				"call":    "StreamHLSm3u8",
			}).Errorln(err.Error())
			return
		}
		if seq >= 6 {
			_, err := c.Writer.Write([]byte(index))
			if err != nil {
				c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
				log.WithFields(logrus.Fields{
					"module":  "http_hls",
					"stream":  streamID,
					"channel": channelID,
					"func":    "HTTPAPIServerStreamHLSM3U8",
					"call":    "Write",
				}).Errorln(err.Error())
				return
			}
			return
		}
		time.Sleep(1 * time.Second)
	}
}

//HTTPAPIServerStreamHLSTS send client ts segment
func HTTPAPIServerStreamHLSTS(c *gin.Context) {

	streamID := c.Param("uuid")
	channelID := c.Param("channel")

	if !Storage.StreamChannelExist(streamID, channelID) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_hls",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamHLSTS",
			"call":    "StreamChannelExist",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}

	codecs, err := Storage.StreamChannelCodecs(streamID, channelID)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_hls",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamHLSTS",
			"call":    "StreamCodecs",
		}).Errorln(err.Error())
		return
	}

	outfile := bytes.NewBuffer([]byte{})
	Muxer := ts.NewMuxer(outfile)
	Muxer.PaddingToMakeCounterCont = true
	err = Muxer.WriteHeader(codecs)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_hls",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamHLSTS",
			"call":    "WriteHeader",
		}).Errorln(err.Error())
		return
	}
	seqData, err := Storage.StreamHLSTS(streamID, channelID, stringToInt(c.Param("seq")))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_hls",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamHLSTS",
			"call":    "StreamHLSTS",
		}).Errorln(err.Error())
		return
	}
	if len(seqData) == 0 {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotHLSSegments.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_hls",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamHLSTS",
			"call":    "seqData",
		}).Errorln(ErrorStreamNotHLSSegments.Error())
		return
	}
	for _, v := range seqData {
		v.CompositionTime = 1
		err = Muxer.WritePacket(*v)
		if err != nil {
			c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
			log.WithFields(logrus.Fields{
				"module":  "http_hls",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamHLSTS",
				"call":    "WritePacket",
			}).Errorln(err.Error())
			return
		}
	}
	err = Muxer.WriteTrailer()
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_hls",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamHLSTS",
			"call":    "WriteTrailer",
		}).Errorln(err.Error())
		return
	}
	_, err = c.Writer.Write(outfile.Bytes())
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_hls",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamHLSTS",
			"call":    "Write",
		}).Errorln(err.Error())
		return
	}

}
