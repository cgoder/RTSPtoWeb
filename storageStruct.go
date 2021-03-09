package main

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/av/pubsub"
	"github.com/sirupsen/logrus"
)

var Storage = NewStreamCore()

//Default stream  type
const (
	MSE = iota
	WEBRTC
	RTSP
)

//Default stream status type
const (
	OFFLINE = iota
	ONLINE
)

//Default stream errors
var (
	Success                         = "success"
	ErrorStreamNotFound             = errors.New("stream not found")
	ErrorStreamAlreadyExists        = errors.New("stream already exists")
	ErrorStreamChannelAlreadyExists = errors.New("stream channel already exists")
	ErrorStreamNotHLSSegments       = errors.New("stream hls not ts seq found")
	ErrorStreamNoVideo              = errors.New("stream no video")
	ErrorStreamNoClients            = errors.New("stream no clients")
	ErrorStreamRestart              = errors.New("stream restart")
	ErrorStreamStopCoreSignal       = errors.New("stream stop core signal")
	ErrorStreamStopRTSPSignal       = errors.New("stream stop rtsp signal")
	ErrorStreamChannelNotFound      = errors.New("stream channel not found")
	ErrorStreamChannelCodecNotFound = errors.New("stream channel codec not ready, possible stream offline")
	ErrorStreamChannelCodecUpdate   = errors.New("stream channel codec update")
	ErrorStreamsLen0                = errors.New("streams len zero")
)

//StorageST main storage struct
type StorageST struct {
	mutex   sync.RWMutex
	Server  ServerST            `json:"server" groups:"api,config"`
	Streams map[string]StreamST `json:"streams" groups:"api,config"`
}

//ServerST server storage section
type ServerST struct {
	Debug        bool         `json:"debug" groups:"api,config"`
	LogLevel     logrus.Level `json:"log_level" groups:"api,config"`
	HTTPDemo     bool         `json:"http_demo" groups:"api,config"`
	HTTPDebug    bool         `json:"http_debug" groups:"api,config"`
	HTTPLogin    string       `json:"http_login" groups:"api,config"`
	HTTPPassword string       `json:"http_password" groups:"api,config"`
	HTTPDir      string       `json:"http_dir" groups:"api,config"`
	HTTPPort     string       `json:"http_port" groups:"api,config"`
	RTSPPort     string       `json:"rtsp_port" groups:"api,config"`
}

//ServerST stream storage section
type StreamST struct {
	Name     string               `json:"name,omitempty" groups:"api,config"`
	Channels map[string]ChannelST `json:"channels,omitempty" groups:"api,config"`
}

type ChannelST struct {
	Name string `json:"name,omitempty" groups:"api,config"`
	URL  string `json:"url,omitempty" groups:"api,config"`
	// auto streaming flag. FALSE==auto. default:false.
	OnDemand bool `json:"on_demand,omitempty" groups:"api,config"`
	Debug    bool `json:"debug,omitempty" groups:"api,config"`
	// online/offline. means channel is streaming.
	Status int `json:"status,omitempty" groups:"api"`

	// codecs []av.CodecData
	// sdp []byte

	// channel update. or codec update.
	updated chan bool
	cond    *sync.Cond
	// opreation signal.
	signals chan int

	// HLS
	hlsSegmentBuffer map[int]Segment
	hlsSegmentNumber int

	// sub clients
	clients map[string]*ClientST

	// av
	av AvST

	// unuseful
	runLock bool
	ack     time.Time
}

//ClientST client storage section
type ClientST struct {
	mode              int
	signals           chan int
	outgoingAVPacket  chan *av.Packet
	outgoingRTPPacket chan *[]byte
	socket            net.Conn
}

//Segment HLS cache section
type Segment struct {
	dur  time.Duration
	data []*av.Packet
}

//AvST av
type AvST struct {
	avCodecs []av.CodecData
	// add for multi clients
	avQue *pubsub.Queue
	sdp   []byte
}
