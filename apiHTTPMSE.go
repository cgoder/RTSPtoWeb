package main

import (
	"context"
	"io"
	"time"

	"github.com/cgoder/vdk/format/mp4f"
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

	log.Println("mse++++++")
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
	cid, avChanR, err := Storage.ClientAdd(streamID, channelID, MSE)
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
	// log.Println("add client. clients: ", Storage.ClientCount(streamID, channelID))
	// defer Storage.ClientDelete(streamID, cid, channelID)
	defer func() {
		Storage.ClientDelete(streamID, cid, channelID)
		// log.Println("del client. clients: ", Storage.ClientCount(streamID, channelID))
	}()
	log.Println("mse++++++ ok")

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

	// make writeable chan for read av.pkt
	eofSignal := make(chan interface{}, 1)
	defer func() {
		eofSignal <- "wsEOF"
	}()

	go func() {

		var videoStart bool

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
			case avPkt := <-avChanR:
				if avPkt.IsKeyFrame {
					// log.Println("MSE got keyFrame. ", avPkt.Time, len(avPkt.Data))
					videoStart = true
				}

				if !videoStart {
					continue
				}

				t1 := time.Now()
				ready, buf, err := muxerMSE.WritePacket(*avPkt, false)
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
				t2 := time.Now()
				if t2.Sub(t1) > 10*time.Millisecond {

					log.Println("mse write pkt cost: ", t2.Sub(t1).String())
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
				t3 := time.Now()
				if t3.Sub(t2) > 10*time.Millisecond {
					log.Println("mse send pkt cost: ", t3.Sub(t2).String())
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamMSE",
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
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "WS.Receive",
					}).Infoln("EXIT! WS got exit signal.", err)
				} else {
					log.WithFields(logrus.Fields{
						"module":  "http_mse",
						"stream":  streamID,
						"channel": channelID,
						"func":    "HTTPAPIServerStreamMSE",
						"call":    "WS.Receive",
					}).Errorln(err.Error())
				}
				// eofSignal <- "wsEOF"
				return
			}

			log.WithFields(logrus.Fields{
				"module":  "http_mse",
				"stream":  streamID,
				"channel": channelID,
				"func":    "HTTPAPIServerStreamMSE",
				"call":    "recv avpkt",
			}).Debugln("WS recv msg: ", message)
		}
	}

}
