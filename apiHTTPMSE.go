package main

import (
	"context"
	"io"
	"time"

	"github.com/deepch/vdk/format/mp4f"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

var timeout_novideo time.Duration = 10
var tiemout_ws time.Duration = 10

//HTTPAPIServerStreamMSE func
func HTTPAPIServerStreamMSE(ws *websocket.Conn) {
	streamID := ws.Request().FormValue("uuid")
	channelID := ws.Request().FormValue("channel")

	ctx, cancel := context.WithCancel(context.Background())

	// close websocket. and release goroutine.
	defer func() {
		_ = ws.Close()
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "MSE",
		}).Debugln("MSE Exit")

		cancel()
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "recv avpkt",
		}).Infoln("Cancel av send goroutine.")
	}()

	// check stream status
	// if !Storage.StreamChannelExist(streamID, channelID)
	ch, err := Storage.StreamChannelGet(streamID, channelID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamChannelGet",
		}).Errorln(ErrorStreamNotFound.Error())
		return
	}

	// streaming
	err = Storage.StreamChannelRun(ctx, streamID, channelID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamChannelRun",
		}).Errorln(err)
		return
	}
	log.WithFields(logrus.Fields{
		"module":  "http_mse",
		"stream":  streamID,
		"channel": channelID,
		"func":    "HTTPAPIServerStreamMSE",
		"call":    "StreamChannelRun",
	}).Debugln("play stream ---> ", streamID, channelID)

	// get stream av.Codec
	codecs, err := Storage.StreamChannelCodecs(streamID, channelID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "StreamCodecs",
		}).Errorln(err.Error())
		return
	}

	// add client/player
	cid, avChanR, _, err := Storage.ClientAdd(streamID, channelID, MSE)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "ClientAdd",
		}).Errorln(err.Error())
		return
	}
	log.Println("add client. clients: ", Storage.ClientCount(streamID, channelID))
	// defer Storage.ClientDelete(streamID, cid, channelID)
	defer func() {
		Storage.ClientDelete(streamID, cid, channelID)
		log.Println("del client. clients: ", Storage.ClientCount(streamID, channelID))
	}()

	// set websocket timeout for write av.Pkt
	err = ws.SetWriteDeadline(time.Now().Add(tiemout_ws * time.Second))
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "SetWriteDeadline",
		}).Errorln(err.Error())
		return
	}

	// init MSE muxer
	muxerMSE := mp4f.NewMuxer(nil)
	err = muxerMSE.WriteHeader(codecs)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "WriteHeader",
		}).Errorln(err.Error())
		return
	}
	meta, init := muxerMSE.GetInit(codecs)
	err = websocket.Message.Send(ws, append([]byte{9}, meta...))
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "Send",
		}).Errorln(err.Error())
		return
	}
	err = websocket.Message.Send(ws, init)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "HTTPAPIServerStreamMSE",
			"call":    "Send MSE meta",
		}).Errorln(err.Error())
		return
	}

	// make writeable chan for read av.pkt
	eofSignal := make(chan interface{}, 1)

	// creat recv av.pkt goroutine
	t1 := time.Now().UTC()

	go wsCheck(ctx, streamID, channelID, ws, eofSignal)
	var videoStart bool
	noVideo := time.NewTimer(time.Duration(timeout_novideo) * time.Second)

	for {
		select {
		case <-ctx.Done():
			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamMSE",
				"call":    "context.Done",
			}).Debugln(ctx.Err())
			return
		case <-eofSignal:
			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamMSE",
				"call":    "got eof signal.",
			}).Debugln("got eof signal.")
			return
		case <-noVideo.C:
			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamMSE",
				"call":    "ErrorStreamNoVideo",
			}).Errorln(ErrorStreamNoVideo.Error())
			log.Println("no video timer. ", time.Now().Sub(t1).String())
			return
		case avPkt := <-avChanR:
			if avPkt.IsKeyFrame {
				log.Println("MSE got avPkt. ", avPkt.IsKeyFrame, len(avPkt.Data))
				videoStart = true
			}
			noVideo.Reset(time.Duration(timeout_novideo) * time.Second)
			t1 = time.Now().UTC()

			if !videoStart {
				continue
			}

			ready, buf, err := muxerMSE.WritePacket(*avPkt, true)
			if err != nil {
				log.WithFields(logrus.Fields{
					"module":  "http_mse",
					"stream":  streamID,
					"channel": channelID,
					"func":    "HTTPAPIServerStreamMSE",
					"call":    "WritePacket",
				}).Errorln(err.Error())
				return
			}
			if ready {
				if ch.Debug && avPkt.IsKeyFrame {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "recv avpkt",
					}).Debugf("Send frame, key:%v, len:%v, DTS:%v, Dur:%v", avPkt.IsKeyFrame, len(buf), avPkt.Time, avPkt.Duration)
				}

				err := ws.SetWriteDeadline(time.Now().Add(tiemout_ws * time.Second))
				if err != nil {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "SetWriteDeadline",
					}).Errorln(err.Error())
					return
				}
				err = websocket.Message.Send(ws, buf)
				if err != nil {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "Send MSE AV",
					}).Errorln(err.Error())
					return
				}
			} else {
				if ch.Debug && len(buf) > 0 {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "recv avpkt",
					}).Debugf("Drop frame, key:%v, len:%v, DTS:%v, Dur:%v", avPkt.IsKeyFrame, len(buf), avPkt.Time, avPkt.Duration)
				}
			}

		}
	}

}

func wsCheck(ctx context.Context, streamID string, channelID string, ws *websocket.Conn, eofSignal chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  streamID,
				"channel": channelID,
				"func":    "wsCheck",
				"call":    "ws exit by cancel.",
			}).Debugln(ctx.Err())
			return
		default:
			var message string
			err := websocket.Message.Receive(ws, &message)
			if err != nil {
				if err == io.EOF {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "wsCheck",
						"call":    "WS.Receive",
					}).Infoln("EXIT! WS got exit signal.")
				} else {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "wsCheck",
						"call":    "WS.Receive",
					}).Errorln(err.Error())
				}
				eofSignal <- "wsEOF"
				return
			}

			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  streamID,
				"channel": channelID,
				"func":    "wsCheck",
				"call":    "recv avpkt",
			}).Debugln("WS recv msg: ", message)
		}
	}
}
