package main

import (
	"context"
	"time"

	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

//HTTPAPIServerStreamWebRTC stream video over WebRTC
func HTTPAPIServerStreamWebRTC(c *gin.Context) {
	streamID := c.Param("uuid")
	channelID := c.Param("channel")

	ctx, cancel := context.WithCancel(context.Background())
	// close websocket. and release goroutine.
	defer func() {
		// _ = ws.Close()
		// c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "WebRTC",
		}).Debugln("WebRTC Exit")

		cancel()
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "recv avpkt",
		}).Infoln("Cancell av send goroutine.")
	}()

	// check stream status
	// if !Storage.StreamChannelExist(streamID, channelID)
	ch, err := Storage.StreamChannelGet(streamID, channelID)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "StreamChannelGet",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}

	// streaming
	err = Storage.StreamChannelRun(ctx, streamID, channelID)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "StreamChannelRun",
		}).Errorln(err)
		return
	}
	log.WithFields(logrus.Fields{
		"module":  "http_webrtc",
		"stream":  streamID,
		"channel": channelID,
		"func":    "HTTPAPIServerStreamWebRTC",
		"call":    "StreamChannelRun",
	}).Debugln("play stream ---> ", streamID, channelID)

	// get stream av.Codec
	codecs, err := Storage.StreamChannelCodecs(streamID, channelID)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "StreamCodecs",
		}).Errorln(err.Error())
		return
	}

	// add client/player
	cid, avChanR, _, err := Storage.ClientAdd(streamID, channelID, WEBRTC)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "ClientAdd",
		}).Errorln(err.Error())
		return
	}
	defer Storage.ClientDelete(streamID, cid, channelID)

	//webrtc init.
	muxerWebRTC := webrtc.NewMuxer(webrtc.Options{})
	answer, err := muxerWebRTC.WriteHeader(codecs, c.PostForm("data"))
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "WriteHeader",
		}).Errorln(err.Error())
		return
	}
	_, err = c.Writer.Write([]byte(answer))
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.WithFields(logrus.Fields{
			"module":  "http_webrtc",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamWebRTC",
			"call":    "Write",
		}).Errorln(err.Error())
		return
	}

	// make writeable chan for read av.pkt
	eofSignal := make(chan interface{}, 1)

	// creat recv av.pkt goroutine
	go webCheck(ctx, streamID, channelID, c, eofSignal)

	var videoStart bool
	noVideo := time.NewTimer(time.Duration(timeout_novideo) * time.Second)

	for {
		select {
		case <-ctx.Done():
			c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNoVideo.Error()})
			log.WithFields(logrus.Fields{
				"module":  "http_webrtc",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamWebRTC",
				"call":    "context.Done",
			}).Debugln(ctx.Err())
			return
		case <-eofSignal:
			c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNoVideo.Error()})
			log.WithFields(logrus.Fields{
				"module":  "http_webrtc",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamWebRTC",
				"call":    "got eof signal.",
			}).Debugln("got eof signal.")
			return
		case <-noVideo.C:
			c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNoVideo.Error()})
			log.WithFields(logrus.Fields{
				"module":  "http_webrtc",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamWebRTC",
				"call":    "ErrorStreamNoVideo",
			}).Errorln(ErrorStreamNoVideo.Error())
			return
		case avPkt := <-avChanR:
			// log.Println("got avPkt. ", avPkt.IsKeyFrame, len(avPkt.Data))
			if avPkt.IsKeyFrame {
				videoStart = true
			}
			noVideo.Reset(time.Duration(timeout_novideo) * time.Second)

			if !videoStart {
				continue
			}

			err = muxerWebRTC.WritePacket(*avPkt)
			if err != nil {
				log.WithFields(logrus.Fields{
					"module":  "http_webrtc",
					"stream":  streamID,
					"channel": channelID,
					"func":    "HTTPAPIServerStreamWebRTC",
					"call":    "WritePacket",
				}).Errorln(err.Error())
				return
			}

			if ch.Debug && avPkt.IsKeyFrame {
				log.WithFields(logrus.Fields{
					"module":  "http_webrtc",
					"stream":  streamID,
					"channel": channelID,
					"func":    "HTTPAPIServerStreamWebRTC",
					"call":    "recv avpkt",
				}).Debugf("Send frame, key:%v, len:%v, DTS:%v, Dur:%v", avPkt.IsKeyFrame, len(avPkt.Data), avPkt.Time, avPkt.Duration)
			}

		}
	}

}

func webCheck(ctx context.Context, streamID string, channelID string, c *gin.Context, eofSignal chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			log.WithFields(logrus.Fields{
				"module":  "http_webrtc",
				"stream":  streamID,
				"channel": channelID,
				"func":    "webCheck",
				"call":    "context.Done",
			}).Debugln(ctx.Err())
			return
			// default:
			// 	var message string
			// 	err := websocket.Message.Receive(ws, &message)
			// 	if err != nil {
			// 		if err == io.EOF {
			// 			log.WithFields(logrus.Fields{
			// 				"module":  "http_webrtc",
			// 				"stream":  streamID,
			// 				"channel": channelID,
			// 				"func":    "webCheck",
			// 				"call":    "WS.Receive",
			// 			}).Infoln("EXIT! WS got exit signal.")
			// 		} else {
			// 			log.WithFields(logrus.Fields{
			// 				"module":  "http_webrtc",
			// 				"stream":  streamID,
			// 				"channel": channelID,
			// 				"func":    "webCheck",
			// 				"call":    "WS.Receive",
			// 			}).Errorln(err.Error())
			// 		}
			// 		eofSignal <- "wsEOF"
			// 		return
			// 	}

			// 	log.WithFields(logrus.Fields{
			// 		"module":  "http_webrtc",
			// 		"stream":  streamID,
			// 		"channel": channelID,
			// 		"func":    "webCheck",
			// 		"call":    "recv avpkt",
			// 	}).Debugln("WS recv msg: ", message)
		}
	}
}
