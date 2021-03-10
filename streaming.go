package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/rtmp"
	"github.com/deepch/vdk/format/rtspv2"
	"github.com/sirupsen/logrus"
)

//StreamRunAll run all stream/channel.
func StreamRunAll() {
	ctx := context.Background()

	Storage.mutex.Lock()
	defer Storage.mutex.Unlock()
	for stID, sts := range Storage.Streams {
		for chID, ch := range sts.Channels {
			if !ch.OnDemand {
				go StreamChannelRun(ctx, stID, chID)
			}
		}
	}

}

//StreamChannelRun run stream/channel.
func StreamChannelRun(ctx context.Context, streamID string, channelID string) error {
	// get stream channel
	channel, err := Storage.StreamChannelGet(streamID, channelID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "streaming",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamChannelRun",
			"call":    "StreamChannelGet",
		}).Errorln("Exit", err)
		return err
	}

	// channle is streaming?
	if channel.Status == ONLINE {
		log.WithFields(logrus.Fields{
			"module":  "streaming",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamChannelRun",
			"call":    "channel.Status",
		}).Infoln("channel is streaming...")

		return nil
	}

	Storage.StreamChannelStatus(streamID, channelID, ONLINE)

	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamChannelRun",
		"call":    "StreamChannelRun",
	}).Infoln("Run stream-> ", streamID, channelID)

	// real stream run
	go readPktByProtocol(ctx, streamID, channelID, channel)
	go writePktToClient(ctx, streamID, channelID, channel)

	return nil

}

//readWritePkt write av.Pkt for all clients. which from channel.av.queue.
func writePktToClient(ctx context.Context, streamID string, channelID string, channel *ChannelST) error {
	var videoStart bool
	clients := channel.clients

	// get av.Pkt from queue.
	cursor := channel.av.avQue.Latest()

	// checkClients := time.NewTimer(time.Duration(timeoutClientCheck) * time.Second)

	for {
		select {
		case <-ctx.Done():
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "writePktToClient",
				"call":    "ctx.Done()",
			}).Debugln("End write avPkt by cancel. ")
			return nil
		// //Check stream have clients
		// case <-checkClients.C:
		// 	cCnt := Storage.ClientCount(streamID, channelID)
		// 	if cCnt == 0 {
		// 		log.WithFields(logrus.Fields{
		// 			"module":  "core",
		// 			"stream":  streamID,
		// 			"channel": channelID,
		// 			"func":    "writePktToClient",
		// 			"call":    "ClientCount",
		// 		}).Debugln("Stream close has no client. ")
		// 		return ErrorStreamNoClients
		// 	}
		// 	log.Println("clients: ", cCnt)
		// 	checkClients.Reset(time.Duration(timeoutClientCheck) * time.Second)
		//Read core signals
		// case signals := <-channel.signals:
		// 	switch signals {
		// 	case SignalStreamStop:
		// 		return ErrorStreamStopCoreSignal
		// 	case SignalStreamRestart:
		// 		//TODO:
		// 		// return 0, ErrorStreamRestart
		// 	case SignalStreamCodecUpdate:
		// 		//TODO:
		// 		// return 0, ErrorStreamChannelCodecUpdate
		// 	}
		//Read av.Pkt,and proxy for all clients.
		// TODO: av.Pkt be save file here.
		default:
			packet, err := cursor.ReadPacket()
			if err != nil {
				log.WithFields(logrus.Fields{
					"module":  "core",
					"stream":  streamID,
					"channel": channelID,
					"func":    "StreamServerRunStreamRtmp",
					"call":    "ReadPacket",
				}).Errorln("ReadPacket error ", err)
				continue
			}

			// checkAvRead.Reset(time.Duration(timeoutAvReadCheck) * time.Second)

			if packet.IsKeyFrame {
				log.Println("Write keyframe to client. ", packet.Idx, len(packet.Data))
				videoStart = true
			}

			if !videoStart {
				continue
			}

			writePktToAllClient(clients, &packet)

		}
	}

}

//readPktByProtocol read av.Pkt from av.Source.
func readPktByProtocol(ctx context.Context, streamID string, channelID string, channel *ChannelST) error {
	if strings.HasPrefix(channel.URL, "rtmp://") {
		_, err := streamRtmp(ctx, streamID, channelID, channel)
		if err != nil {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "streamByProtocol",
				"call":    "streamRtmp",
			}).Errorln(err)
		}
		return err
	} else if strings.HasPrefix(channel.URL, "rtsp://") {
		_, err := streamRtsp(ctx, streamID, channelID, channel)
		if err != nil {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "streamByProtocol",
				"call":    "streamRtsp",
			}).Errorln(err)
		}
		return err
	} else {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "streamByProtocol",
			"call":    "protocal select",
		}).Errorln("Unsupport protocol. ", channel.URL)
		return errors.New("Unsupport protocol")
	}
}

//streamRtmp read av.Pkt from rtmp stream to av.avQue.
func streamRtmp(ctx context.Context, streamID string, channelID string, channel *ChannelST) (int, error) {
	var RTMPConn *rtmp.Conn
	var err error

	// rtmp connect.
	if err := func() error {
		// rtmp client dial
		RTMPConn, err = rtmp.Dial(channel.URL)

		if err != nil {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStreamRtmp",
				"call":    "rtmp.Dial",
			}).Errorln("RTMP Dial ---> ", JsonFormat(RTMPConn.URL), err)
			return err
		}
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamServerRunStreamRtmp",
			"call":    "Start",
		}).Debugln("RTMP Conn---> ", JsonFormat(RTMPConn.URL))

		// get av.Codec
		t1 := time.Now().Local().UTC()
		streams, err := RTMPConn.Streams()
		if err != nil {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStreamRtmp",
				"call":    "RTMPConn.Streams",
			}).Errorln("RTMP get stream codec err. ", err)
			return err
		}

		// update av.Codec
		if len(streams) > 0 {
			Storage.StreamChannelCodecsUpdate(streamID, channelID, streams, nil)
			// channel.updated <- true
			channel.cond.Broadcast()

			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStreamRtmp",
				"call":    "RTMPConn.Streams",
			}).Debugln("rtmp get stream codec DONE! time: ", time.Now().Local().UTC().Sub(t1).String())
		} else {
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStreamRtmp",
				"call":    "RTMPConn.Streams",
			}).Errorln("rtmp get stream codec fail! time: ", time.Now().Local().UTC().Sub(t1).String())
			return ErrorStreamChannelCodecNotFound
		}

		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamServerRunStreamRtmp",
			"call":    "Start",
		}).Debugln("Success connection RTMP")

		return nil
	}(); err != nil {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "streamRtmp",
			"call":    "rtmp connect",
		}).Errorln("RTMP connect fail. ---> ", JsonFormat(channel.URL), err)
		return 0, err
	}
	// release hls cache
	defer func() {
		RTMPConn.Close()
		Storage.StreamChannelCodecsUpdate(streamID, channelID, nil, nil)
		Storage.StreamChannelStatus(streamID, channelID, OFFLINE)
		Storage.StreamHLSFlush(streamID, channelID)
	}()

	checkClients := time.NewTimer(time.Duration(timeoutClientCheck) * time.Second)
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet
	var pktCnt int
	for {
		select {
		case <-ctx.Done():
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "StreamServerRunStreamRtmp",
				"call":    "ctx.Done()",
			}).Debugln("Stream close by cancel. ")
			return 0, nil
		//Check stream have clients
		case <-checkClients.C:
			cCnt := Storage.ClientCount(streamID, channelID)
			if cCnt == 0 {
				log.WithFields(logrus.Fields{
					"module":  "core",
					"stream":  streamID,
					"channel": channelID,
					"func":    "StreamServerRunStreamRtmp",
					"call":    "ClientCount",
				}).Debugln("Stream close has no client. ")
				return 0, ErrorStreamNoClients
			}
			log.Println("clients: ", cCnt)
			if b := checkClients.Reset(time.Duration(timeoutClientCheck) * time.Second); !b {
				log.Println("checkClients timer reset err")
			}
		//Read core signals
		case signals := <-channel.signals:
			switch signals {
			case SignalStreamStop:
				return 0, ErrorStreamStopCoreSignal
			case SignalStreamRestart:
				//TODO:
				// return 0, ErrorStreamRestart
			case SignalStreamCodecUpdate:
				//TODO:
				// return 0, ErrorStreamChannelCodecUpdate
			}
		//Read av.Pkt,and proxy for all clients.
		// TODO: av.Pkt be save file here.
		default:
			avPkt, err := RTMPConn.ReadPacket()
			if err != nil {
				log.WithFields(logrus.Fields{
					"module":  "core",
					"stream":  streamID,
					"channel": channelID,
					"func":    "StreamServerRunStreamRtmp",
					"call":    "ReadPacket",
				}).Errorln("ReadPacket error ", err)
				continue
			}

			pktCnt++
			// if avPkt.IsKeyFrame {
			// 	log.Println("Write keyframe to queue. ", pktCnt, avPkt.Idx, len(avPkt.Data))
			// }

			// write av.Pkt to avQue
			channel.av.avQue.WritePacket(avPkt)

			if avPkt.IsKeyFrame {
				if preKeyTS > 0 {
					Storage.StreamHLSAdd(streamID, channelID, Seq, avPkt.Time-preKeyTS)
					Seq = []*av.Packet{}
				}
				preKeyTS = avPkt.Time
			}
			Seq = append(Seq, &avPkt)

		}
	}
}

//streamRtsp read av.Pkt from rtmp stream to av.avQue.
func streamRtsp(ctx context.Context, streamID string, channelID string, channel *ChannelST) (int, error) {
	t1 := time.Now().Local().UTC()
	// rtsp client dial
	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: channel.URL, DisableAudio: true, DialTimeout: 3 * time.Second, ReadWriteTimeout: 5 * time.Second, Debug: channel.Debug, OutgoingProxy: true})
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "streamRtsp",
			"call":    "rtspv2.Dial",
		}).Errorln("RTSPClient.Dial fail. ", err)
		return 0, err
	}
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "streamRtsp",
		"call":    "rtspv2.Dial",
	}).Debugln("RTSPClient.SDPRaw---> ", JsonFormat(RTSPClient.SDPRaw))

	if len(RTSPClient.CodecData) > 0 {
		Storage.StreamChannelCodecsUpdate(streamID, channelID, RTSPClient.CodecData, RTSPClient.SDPRaw)
		defer func() {
			Storage.StreamChannelCodecsUpdate(streamID, channelID, nil, nil)
		}()

		// channel.updated <- true
		channel.cond.Broadcast()

		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "streamRtsp",
			"call":    "rtspv2.Dial",
		}).Debugln("rtsp get stream codec DONE! time: ", time.Now().Local().UTC().Sub(t1).String())
	} else {
		log.WithFields(logrus.Fields{
			"module":  "core",
			"stream":  streamID,
			"channel": channelID,
			"func":    "streamRtsp",
			"call":    "rtspv2.Dial",
		}).Errorln("rtsp get stream codec fail! time: ", time.Now().Local().UTC().Sub(t1).String())
		return 0, ErrorStreamChannelCodecNotFound
	}

	// // stream status update
	// Storage.StreamChannelStatus(streamID, channelID, ONLINE)
	// defer Storage.StreamChannelStatus(streamID, channelID, OFFLINE)
	log.WithFields(logrus.Fields{
		"module":  "core",
		"stream":  streamID,
		"channel": channelID,
		"func":    "streamRtsp",
		"call":    "StreamChannelStatus",
	}).Debugln("Success connection RTSP")

	// release hls cache
	defer func() {
		Storage.StreamChannelStatus(streamID, channelID, OFFLINE)
		Storage.StreamHLSFlush(streamID, channelID)
	}()

	// checkClients := time.NewTimer(time.Duration(timeoutClientCheck) * time.Second)
	var preKeyTS = time.Duration(0)
	var Seq []*av.Packet

	for {
		select {
		case <-ctx.Done():
			log.WithFields(logrus.Fields{
				"module":  "core",
				"stream":  streamID,
				"channel": channelID,
				"func":    "streamRtsp",
				"call":    "ctx.Done()",
			}).Debugln("Stream close by cancel. ")
			return 0, nil
		//Read core signals
		case signals := <-channel.signals:
			switch signals {
			case SignalStreamStop:
				return 0, ErrorStreamStopCoreSignal
			case SignalStreamRestart:
				//TODO:
				// return 0, ErrorStreamRestart
			case SignalStreamCodecUpdate:
				//TODO:
				// return 0, ErrorStreamChannelCodecUpdate
			}
		//Read rtsp signals
		case signals := <-RTSPClient.Signals:
			switch signals {
			case rtspv2.SignalCodecUpdate:
				Storage.StreamChannelCodecsUpdate(streamID, channelID, RTSPClient.CodecData, RTSPClient.SDPRaw)
				log.WithFields(logrus.Fields{
					"module":  "core",
					"stream":  streamID,
					"channel": channelID,
					"func":    "streamRtsp",
					"call":    "RTSPClient.Signals",
				}).Debugln("Stream codec update. ")
			case rtspv2.SignalStreamRTPStop:
				log.WithFields(logrus.Fields{
					"module":  "core",
					"stream":  streamID,
					"channel": channelID,
					"func":    "streamRtsp",
					"call":    "RTSPClient.Signals",
				}).Errorln(ErrorStreamStopRTSPSignal)
				return 0, ErrorStreamStopRTSPSignal
			}
		// read rtp.Pkt for cast all clients. MUST read from OutgoingProxyQueue.
		case <-RTSPClient.OutgoingProxyQueue:
			// Storage.StreamChannelCastProxy(streamID, channelID, packetRTP)
		// read av.Pkt for cast all clients.
		case avPkt := <-RTSPClient.OutgoingPacketQueue:
			// Storage.StreamChannelCast(streamID, channelID, avPkt)
			channel.av.avQue.WritePacket(*avPkt)

			if avPkt.IsKeyFrame {
				if preKeyTS > 0 {
					Storage.StreamHLSAdd(streamID, channelID, Seq, avPkt.Time-preKeyTS)
					Seq = []*av.Packet{}
				}
				preKeyTS = avPkt.Time
			}
			Seq = append(Seq, avPkt)
		}
	}
}

//writePktToAllClient cast av.Pkt to all clients.
func writePktToAllClient(clients map[string]*ClientST, avPkt *av.Packet) {

	if len(clients) > 0 {
		for _, client := range clients {
			if client.mode == RTSP {
				if len(client.outgoingRTPPacket) < lenAvPacketQueue {
					client.outgoingRTPPacket <- &avPkt.Data
				} else if len(client.signals) < lenClientSignalQueue {
					//send stop signals to client
					client.signals <- SignalStreamStop
				}
			} else {
				if len(client.outgoingAVPacket) < lenAvPacketQueue {
					// log.Println("w2c ", avPkt.Idx, avPkt.IsKeyFrame)
					client.outgoingAVPacket <- avPkt
				} else if len(client.signals) < lenClientSignalQueue {
					//send stop signals to client
					client.signals <- SignalStreamStop
				}
			}
		}
	}
}
