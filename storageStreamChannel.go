package main

import (
	"context"
	"sync"
	"time"

	"github.com/cgoder/vdk/av"
	"github.com/cgoder/vdk/av/pubsub"
	"github.com/sirupsen/logrus"
)

//StreamChannelNew new channel obj.
func StreamChannelNew(url string, name string) *ChannelST {
	var tmpCh ChannelST
	//gen uuid
	tmpCh.URL = url
	tmpCh.UUID = GenerateUUID()
	tmpCh.Name = name
	if tmpCh.Name == "" {
		tmpCh.Name = tmpCh.UUID
	}

	//init source stream
	var source AvStream
	source.avQue = pubsub.NewQueue()
	tmpCh.source = &source
	//init client's
	tmpCh.clients = make(map[string]*ClientST)

	// make chan for av.Codec update
	tmpCh.cond = sync.NewCond(&sync.Mutex{})
	//make signals buffer chain
	tmpCh.signals = make(chan int, 10)

	//make hls buffer
	tmpCh.hlsSegmentBuffer = make(map[int]*Segment)
	tmpCh.hlsSegmentNumber = 0

	//init debug
	tmpCh.OnDemand = true
	tmpCh.Debug = false

	return &tmpCh

}

//StreamChannelRelease renew channel for GC.
func StreamChannelRelease(ch *ChannelST) {
	var tmpCh ChannelST

	//release
	ch.source.avQue.Close()
	ch.source.avQue = nil
	var source AvStream
	ch.source = &source
	ch.clients = make(map[string]*ClientST)
	ch.signals = make(chan int, 10)
	ch.hlsSegmentBuffer = make(map[int]*Segment)
	ch.hlsSegmentNumber = 0
	ch.OnDemand = true
	ch.Debug = false

	ch = &tmpCh

}

//StreamChannelRunAll run all stream go
func (obj *StorageST) StreamChannelRunAll(ctx context.Context) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	for k, v := range obj.Streams {
		for ks, vs := range v.Channels {
			if !vs.OnDemand {
				// go StreamServerRunStreamDo(ctx, k, ks)
				go StreamChannelRun(ctx, k, ks)
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
	ch, err := Storage.StreamChannelGet(streamID, channelID)
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

	if ch.source.status != ONLINE {
		// go StreamServerRunStreamDo(ctx, streamID, channelID)
		go StreamChannelRun(context.Background(), streamID, channelID)
	} else {
		log.WithFields(logrus.Fields{
			"module":  "StreamChannel",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamChannelRun",
			"call":    "StreamChannelRun",
		}).Debugln("stream is running... clients:", len(ch.clients))
		// return ErrorStreamAlreadyRunning
	}

	return nil
}

//StreamChannelStop stop stream
func (obj *StorageST) StreamChannelStop(streamID string, channelID string) error {
	// get stream channel
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	if streamID == "" && channelID == "" {
		for _, program := range obj.Streams {
			for _, ch := range program.Channels {
				//TODO: stop stream.

				//set stream status
				ch.source.status = OFFLINE
			}
		}
		return nil
	} else {
		if channelID != "" {
			ch, ok := obj.Streams[streamID].Channels[channelID]
			if !ok {
				return ErrorStreamChannelNotFound
			}
			//TODO: stop stream.

			//set stream status
			ch.source.status = OFFLINE
		} else {
			program, ok := obj.Streams[streamID]
			if !ok {
				return ErrorStreamNotFound
			}
			for _, ch := range program.Channels {
				//TODO: stop stream.

				//set stream status
				ch.source.status = OFFLINE
			}
		}
	}
	return nil
}

//StreamChannelGet get stream
func (obj *StorageST) StreamChannelGet(streamID string, channelID string) (*ChannelST, error) {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	ch, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return nil, ErrorStreamChannelNotFound
	}
	return ch, nil
}

//StreamChannelExist check stream exist
func (obj *StorageST) StreamChannelExist(streamID string, channelID string) bool {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	_, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return false
	}
	return true
}

//StreamChannelReload reload stream
func (obj *StorageST) StreamChannelReload(uuid string, channelID string) error {
	// obj.mutex.RLock()
	// defer obj.mutex.RUnlock()
	// if tmp, ok := obj.Streams[uuid]; ok {
	// 	if channelTmp, ok := tmp.Channels[channelID]; ok {
	// 		channelTmp.signals <- SignalStreamRestart
	// 		return nil
	// 	} else {
	// 		return ErrorStreamChannelNotFound
	// 	}
	// }
	// return ErrorStreamNotFound
	return nil
}

//StreamChannelCodecs get stream codec storage or wait
func (obj *StorageST) StreamChannelCodecs(streamID string, channelID string) ([]av.CodecData, error) {
	obj.mutex.Lock()
	ch, ok := obj.Streams[streamID].Channels[channelID]
	obj.mutex.Unlock()
	if !ok {
		return nil, ErrorStreamChannelNotFound
	}

	if ch.source.status == ONLINE && ch.source.avCodecs != nil {
		// log.WithFields(logrus.Fields{
		// 	"module":  "StreamChannel",
		// 	"stream":  streamID,
		// 	"channel": channelID,
		// 	"func":    "StreamChannelCodecs",
		// 	"call":    "chan.updated",
		// }).Debugln("Got old codec!")
		return ch.source.avCodecs, nil
	} else {
		log.WithFields(logrus.Fields{
			"module":  "http_mse",
			"stream":  streamID,
			"channel": channelID,
			"func":    "StreamChannelCodecs",
			"call":    "Get Stream codec ...",
		}).Debugf("Get Stream codec ...")

		t1 := time.Now().UTC()

		//wait for stream status update, recv cond.Broadcast().
		ch.cond.L.Lock()
		defer ch.cond.L.Unlock()
		ch.cond.Wait()

		obj.mutex.Lock()
		ch, ok := obj.Streams[streamID].Channels[channelID]
		obj.mutex.Unlock()
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

		return ch.source.avCodecs, nil
	}

}

//StreamChannelStatusUpdate change stream status
func (obj *StorageST) StreamChannelStatusUpdate(streamID string, channelID string, status int) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	ch, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return ErrorStreamChannelNotFound
	}

	ch.source.status = status

	return nil
}

//StreamChannelCodecsUpdate update stream codec storage
func (obj *StorageST) StreamChannelCodecsUpdate(streamID string, channelID string, avcodec []av.CodecData, sdp []byte) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	ch, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return ErrorStreamChannelNotFound
	}

	ch.source.sdp = sdp
	ch.source.avCodecs = avcodec

	return nil
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
			if client.protocol == RTSP {
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

		if len(channelTmp.source.sdp) > 0 {
			return channelTmp.source.sdp, nil
		}
		// why sleep?
		time.Sleep(50 * time.Millisecond)
	}
	return nil, ErrorStreamNotFound
}

//StreamChannelAdd add stream
func (obj *StorageST) StreamChannelAdd(ctx context.Context, streamID string, channelID string, ch *ChannelST) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	ch, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return ErrorStreamChannelNotFound
	}

	obj.Streams[streamID].Channels[channelID] = ch

	if !ch.OnDemand {
		// go StreamServerRunStreamDo(ctx, streamID, channelID)
		go StreamChannelRun(ctx, streamID, channelID)
	}

	err := obj.SaveConfig()
	if err != nil {
		return err
	}
	return nil
}

//StreamEdit edit stream
func (obj *StorageST) StreamChannelEdit(ctx context.Context, streamID string, channelID string, ch *ChannelST) error {
	obj.StreamChannelStop(streamID, channelID)

	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	_, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return ErrorStreamChannelNotFound
	}

	obj.Streams[streamID].Channels[channelID] = ch

	if !ch.OnDemand {
		// go StreamServerRunStreamDo(ctx, streamID, channelID)
		go StreamChannelRun(ctx, streamID, channelID)
	}

	err := obj.SaveConfig()
	if err != nil {
		return err
	}
	return nil
}

//StreamChannelDelete stream
func (obj *StorageST) StreamChannelDelete(streamID string, channelID string) error {
	obj.StreamChannelStop(streamID, channelID)

	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	_, ok := obj.Streams[streamID].Channels[channelID]
	if !ok {
		return ErrorStreamChannelNotFound
	}

	delete(obj.Streams[streamID].Channels, channelID)

	err := obj.SaveConfig()
	if err != nil {
		return err
	}
	return nil
}

//StreamChannelCount count online stream channel.
func (obj *StorageST) StreamChannelCount() int {
	var cnt int
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	for _, st := range obj.Streams {
		cnt = cnt + len(st.Channels)
	}

	return cnt
}

//StreamChannelRunning count online stream channel.
func (obj *StorageST) StreamChannelRunning() int {
	var cnt int
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	for _, st := range obj.Streams {
		for _, ch := range st.Channels {
			if ch.source.status == ONLINE {
				cnt++
			}
		}
	}

	return cnt
}
