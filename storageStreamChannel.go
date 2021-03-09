package main

import (
	"context"
	"sync"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/av/pubsub"
	"github.com/sirupsen/logrus"
)

func StreamChannelNew(val ChannelST) ChannelST {
	tmpCh := val
	//make client's
	tmpCh.clients = make(map[string]*ClientST)
	//make last ack
	// tmpCh.ack = time.Now().Add(-255 * time.Hour)
	//make hls buffer
	tmpCh.hlsSegmentBuffer = make(map[int]Segment)
	//make signals buffer chain
	tmpCh.signals = make(chan int, 100)
	// make chan for av.Codec update
	tmpCh.updated = make(chan bool)
	tmpCh.cond = sync.NewCond(&sync.Mutex{})

	var av AvST
	av.avQue = pubsub.NewQueue()
	av.avQue.SetMaxGopCount(1)
	tmpCh.av = av

	return tmpCh
}

//StreamChannelMake check stream exist
func (obj *StorageST) StreamChannelMake(val ChannelST) ChannelST {
	return StreamChannelNew(val)
}

//StreamChannelRunAll run all stream go
func (obj *StorageST) StreamChannelRunAll(ctx context.Context) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	for k, v := range obj.Streams {
		for ks, vs := range v.Channels {
			if !vs.OnDemand {
				// vs.runLock = true
				go StreamServerRunStreamDo(ctx, k, ks)
				v.Channels[ks] = vs
				obj.Streams[k] = v
			}
		}
	}
}

//StreamChannelRun one stream and lock
func (obj *StorageST) StreamChannelRun(ctx context.Context, streamID string, channelID string) error {
	// log.WithFields(logrus.Fields{
	// 	"module":  "StreamChannel",
	// 	"stream":  streamID,
	// 	"channel": channelID,
	// 	"func":    "StreamChannelRun",
	// 	"call":    "StreamChannelRun",
	// }).Debugln("StreamChannelRun ->>>>>>>>>>>>>>>>>")

	// get stream channel
	_, err := Storage.StreamChannelGet(streamID, channelID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"module":  "StreamChannel",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamChannelRun",
			"call":    "StreamChannelGet",
		}).Errorln("Exit", err)
		return ErrorStreamChannelNotFound
	}

	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if streamTmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := streamTmp.Channels[channelID]; ok {
			if channelTmp.Status != ONLINE {
				// go StreamServerRunStreamDo(ctx, streamID, channelID)
				go StreamChannelRun(ctx, streamID, channelID)
			} else {
				log.WithFields(logrus.Fields{
					"module":  "StreamChannel",
					"stream":  streamID,
					"channel": channelID,
					"func":    "StreamChannelRun",
					"call":    "StreamChannelRun",
				}).Debugln("stream is running...")
			}
			return nil
		}
	}
	return nil
}

//StreamChannelGet get stream
func (obj *StorageST) StreamChannelGet(streamID string, channelID string) (*ChannelST, error) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if streamTmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := streamTmp.Channels[channelID]; ok {
			return &channelTmp, nil
		}
		return nil, ErrorStreamChannelNotFound

	}
	return nil, ErrorStreamNotFound

}

//StreamChannelExist check stream exist
func (obj *StorageST) StreamChannelExist(streamID string, channelID string) bool {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if streamTmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := streamTmp.Channels[channelID]; ok {
			channelTmp.ack = time.Now()
			streamTmp.Channels[channelID] = channelTmp
			obj.Streams[streamID] = streamTmp
			return ok
		}
	}
	return false
}

//StreamChannelReload reload stream
func (obj *StorageST) StreamChannelReload(uuid string, channelID string) error {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	if tmp, ok := obj.Streams[uuid]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			channelTmp.signals <- SignalStreamRestart
			return nil
		} else {
			return ErrorStreamChannelNotFound
		}
	}
	return ErrorStreamNotFound
}

//StreamChannelCodecs get stream codec storage or wait
func (obj *StorageST) StreamChannelCodecs(streamID string, channelID string) ([]av.CodecData, error) {
	obj.mutex.RLock()
	tmp, ok := obj.Streams[streamID]
	obj.mutex.RUnlock()
	if !ok {
		return nil, ErrorStreamNotFound
	}
	channelTmp, ok := tmp.Channels[channelID]
	if !ok {
		return nil, ErrorStreamChannelNotFound
	}

	if channelTmp.Status == ONLINE && channelTmp.av.avCodecs != nil {
		// log.WithFields(logrus.Fields{
		// 	"module":  "StreamChannel",
		// 	"stream":  streamID,
		// 	"channel": channelID,
		// 	"func":    "StreamChannelCodecs",
		// 	"call":    "chan.updated",
		// }).Debugln("Got old codec!")
		return channelTmp.av.avCodecs, nil
	}
	// return nil, ErrorStreamChannelCodecNotFound

	log.WithFields(logrus.Fields{
		"module":  "http_mse",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamChannelCodecs",
		"call":    "Get Stream codec ...",
	}).Debugf("Get Stream codec ...")

	t1 := time.Now().UTC()

	channelTmp.cond.L.Lock()
	defer channelTmp.cond.L.Unlock()
	channelTmp.cond.Wait()

	obj.mutex.RLock()
	chTmp, ok := obj.Streams[streamID].Channels[channelID]
	obj.mutex.RUnlock()
	if !ok {
		return nil, ErrorStreamChannelNotFound
	}

	t2 := time.Now().UTC().Sub(t1)
	log.WithFields(logrus.Fields{
		"module":  "http_mse",
		"stream":  streamID,
		"channel": channelID,
		"func":    "StreamChannelCodecs",
		"call":    "chan.updated",
	}).Debugf("Got Stream codec update! cost:%v", t2.String())

	return chTmp.av.avCodecs, nil

	// t1 := time.Now().UTC()
	// timer := time.NewTimer(20 * time.Second)
	// for {
	// 	select {
	// 	case <-timer.C:
	// 		log.WithFields(logrus.Fields{
	// 			"module":  "StreamChannel",
	// 			"stream":  streamID,
	// 			"channel": channelID,
	// 			"func":    "StreamChannelCodecs",
	// 			"call":    "chan.updated",
	// 		}).Errorln("Get codec timeout!")
	// 		return nil, ErrorStreamChannelCodecNotFound
	// 	case <-channelTmp.updated:
	// 		obj.mutex.RLock()
	// 		channelTmp, ok := obj.Streams[streamID].Channels[channelID]
	// 		obj.mutex.RUnlock()
	// 		if !ok {
	// 			return nil, ErrorStreamChannelNotFound
	// 		}

	// 		t2 := time.Now().UTC().Sub(t1)
	// 		log.WithFields(logrus.Fields{
	// 			"module":  "http_mse",
	// 			"stream":  streamID,
	// 			"channel": channelID,
	// 			"func":    "StreamChannelCodecs",
	// 			"call":    "chan.updated",
	// 		}).Debugf("Got Stream codec update! cost:%v", t2.String())

	// 		return channelTmp.av.avCodecs, nil
	// 	}
	// }

}

//StreamChannelStatus change stream status
func (obj *StorageST) StreamChannelStatus(streamID string, channelID string, status int) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			channelTmp.Status = status
			tmp.Channels[channelID] = channelTmp
			obj.Streams[streamID] = tmp
			return nil
		}
		return ErrorStreamChannelNotFound

	}
	return ErrorStreamNotFound
}

//StreamChannelCast broadcast stream av.Pkt
func (obj *StorageST) StreamChannelCast(streamID string, channelID string, val *av.Packet) {
	ch, err := Storage.StreamChannelGet(streamID, channelID)
	if err != nil || ch == nil {
		log.WithFields(logrus.Fields{
			"module":  "StreamChannel",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamChannelCast",
			"call":    "StreamChannelGet",
		}).Errorf("Get channel fail!")
		return
	}

	if len(ch.clients) > 0 {
		for _, client := range ch.clients {
			if client.mode == RTSP {
				continue
			}
			if len(client.outgoingAVPacket) < lenAvPacketQueue {
				client.outgoingAVPacket <- val
			} else if len(client.signals) < lenClientSignalQueue {
				//send stop signals to client
				client.signals <- SignalStreamStop
				//No need close socket only send signal to reader / writer socket closed if client go to offline
				/*
					err := client.socket.Close()
					if err != nil {
						log.WithFields(logrus.Fields{
							"module":  "storage",
							"stream":  streamID,
							"channel": channelID,
							"func":    "CastProxy",
							"call":    "Close",
						}).Errorln(err.Error())
					}
				*/
			}
		}
	}

	// obj.mutex.Lock()
	// defer obj.mutex.Unlock()
	// if tmp, ok := obj.Streams[streamID]; ok {
	// 	if channelTmp, ok := tmp.Channels[channelID]; ok {
	// 		if len(channelTmp.clients) > 0 {
	// 			for _, i2 := range channelTmp.clients {
	// 				if i2.mode == RTSP {
	// 					continue
	// 				}
	// 				if len(i2.outgoingAVPacket) < 1000 {
	// 					i2.outgoingAVPacket <- val
	// 				} else if len(i2.signals) < 10 {
	// 					//send stop signals to client
	// 					i2.signals <- SignalStreamStop
	// 					//No need close socket only send signal to reader / writer socket closed if client go to offline
	// 					/*
	// 						err := i2.socket.Close()
	// 						if err != nil {
	// 							log.WithFields(logrus.Fields{
	// 								"module":  "storage",
	// 								"stream":  streamID,
	// 								"channel": channelID,
	// 								"func":    "CastProxy",
	// 								"call":    "Close",
	// 							}).Errorln(err.Error())
	// 						}
	// 					*/
	// 				}
	// 			}
	// 			channelTmp.ack = time.Now()
	// 			tmp.Channels[channelID] = channelTmp
	// 			obj.Streams[streamID] = tmp
	// 		}
	// 	}
	// }
}

//StreamChannelCastProxy broadcast stream av.RTP
func (obj *StorageST) StreamChannelCastProxy(streamID string, channelID string, val *[]byte) {
	ch, err := Storage.StreamChannelGet(streamID, channelID)
	if err != nil || ch == nil {
		log.WithFields(logrus.Fields{
			"module":  "StreamChannel",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamChannelCast",
			"call":    "StreamChannelGet",
		}).Errorf("Get channel fail!")
		return
	}

	if len(ch.clients) > 0 {
		for _, client := range ch.clients {
			if client.mode != RTSP {
				continue
			}
			if len(client.outgoingRTPPacket) < lenAvPacketQueue {
				client.outgoingRTPPacket <- val
			} else if len(client.signals) < lenClientSignalQueue {
				//send stop signals to client
				client.signals <- SignalStreamStop
				err := client.socket.Close()
				if err != nil {
					log.WithFields(logrus.Fields{
						"module":  "storage",
						"stream":  streamID,
						"channel": channelID,
						"func":    "CastProxy",
						"call":    "Close",
					}).Errorln(err.Error())
				}
			}

		}
	}

	// obj.mutex.Lock()
	// defer obj.mutex.Unlock()
	// if tmp, ok := obj.Streams[streamID]; ok {
	// 	if channelTmp, ok := tmp.Channels[channelID]; ok {
	// 		if len(channelTmp.clients) > 0 {
	// 			for _, i2 := range channelTmp.clients {
	// 				if i2.mode != RTSP {
	// 					continue
	// 				}
	// 				if len(i2.outgoingRTPPacket) < 1000 {
	// 					i2.outgoingRTPPacket <- val
	// 				} else if len(i2.signals) < 10 {
	// 					//send stop signals to client
	// 					i2.signals <- SignalStreamStop
	// 					err := i2.socket.Close()
	// 					if err != nil {
	// 						log.WithFields(logrus.Fields{
	// 							"module":  "storage",
	// 							"stream":  streamID,
	// 							"channel": channelID,
	// 							"func":    "CastProxy",
	// 							"call":    "Close",
	// 						}).Errorln(err.Error())
	// 					}
	// 				}
	// 			}
	// 			channelTmp.ack = time.Now()
	// 			tmp.Channels[channelID] = channelTmp
	// 			obj.Streams[streamID] = tmp
	// 		}
	// 	}
	// }
}

//StreamChannelCodecsUpdate update stream codec storage
func (obj *StorageST) StreamChannelCodecsUpdate(streamID string, channelID string, val []av.CodecData, sdp []byte) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			channelTmp.av.avCodecs = val
			channelTmp.av.sdp = sdp
			tmp.Channels[channelID] = channelTmp
			obj.Streams[streamID] = tmp
		}
	}
}

//StreamChannelSDP codec storage or wait
func (obj *StorageST) StreamChannelSDP(streamID string, channelID string) ([]byte, error) {
	// why for 100 times?
	for i := 0; i < 100; i++ {
		obj.mutex.RLock()
		tmp, ok := obj.Streams[streamID]
		obj.mutex.RUnlock()
		if !ok {
			return nil, ErrorStreamNotFound
		}
		channelTmp, ok := tmp.Channels[channelID]
		if !ok {
			return nil, ErrorStreamChannelNotFound
		}

		if len(channelTmp.av.sdp) > 0 {
			return channelTmp.av.sdp, nil
		}
		// why sleep?
		time.Sleep(50 * time.Millisecond)
	}
	return nil, ErrorStreamNotFound
}

//StreamChannelAdd add stream
func (obj *StorageST) StreamChannelAdd(ctx context.Context, streamID string, channelID string, ch ChannelST) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if _, ok := obj.Streams[streamID]; !ok {
		return ErrorStreamNotFound
	}
	if _, ok := obj.Streams[streamID].Channels[channelID]; ok {
		return ErrorStreamChannelAlreadyExists
	}
	ch = obj.StreamChannelMake(ch)
	obj.Streams[streamID].Channels[channelID] = ch
	if !ch.OnDemand {
		// ch.runLock = true
		go StreamServerRunStreamDo(ctx, streamID, channelID)
	}
	err := obj.SaveConfig()
	if err != nil {
		return err
	}
	return nil
}

//StreamEdit edit stream
func (obj *StorageST) StreamChannelEdit(ctx context.Context, streamID string, channelID string, ch ChannelST) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[streamID]; ok {
		if currentChannel, ok := tmp.Channels[channelID]; ok {
			if currentChannel.Status == ONLINE {
				currentChannel.signals <- SignalStreamStop
			}
			ch = obj.StreamChannelMake(ch)
			obj.Streams[streamID].Channels[channelID] = ch
			if !ch.OnDemand {
				// ch.runLock = true
				go StreamServerRunStreamDo(ctx, streamID, channelID)
			}
			err := obj.SaveConfig()
			if err != nil {
				return err
			}
			return nil
		}
	}
	return ErrorStreamNotFound
}

//StreamChannelDelete stream
func (obj *StorageST) StreamChannelDelete(streamID string, channelID string) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if tmp, ok := obj.Streams[streamID]; ok {
		if channelTmp, ok := tmp.Channels[channelID]; ok {
			if channelTmp.Status == ONLINE {
				channelTmp.signals <- SignalStreamStop
			}
			delete(obj.Streams[streamID].Channels, channelID)
			err := obj.SaveConfig()
			if err != nil {
				return err
			}
			return nil
		}
	}
	return ErrorStreamNotFound
}
