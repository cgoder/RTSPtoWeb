package main

import (
	"errors"
	"sync"
	"time"

	"github.com/cgoder/vdk/av"
	"github.com/cgoder/vdk/av/pubsub"
	"github.com/sirupsen/logrus"
)

var Storage = NewStreamCore()

//Default stream  type
const (
	MSE = iota
	WEBRTC
	RTSP
)

//Default channel/stream status type
const (
	OFFLINE = iota
	ONLINE
)

const (
	STREAM_UNKNOWN = iota
	STREAM_REFRESH
	STREAM_AVCODEC_UPDATE
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
	ErrorStreamAlreadyRunning       = errors.New("stream already running")
)

//StorageST main storage struct
type StorageST struct {
	mutex   sync.RWMutex
	Config  ConfigST              `json:"server" groups:"api,config"`
	Streams map[string]*ProgramST `json:"streams" groups:"api,config"`
}

//ConfigST server storage section
type ConfigST struct {
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

//ConfigST stream storage section
type ProgramST struct {
	UUID     string
	Name     string                `json:"name,omitempty" groups:"api,config"`
	Channels map[string]*ChannelST `json:"channels,omitempty" groups:"api,config"`
}

type ChannelST struct {
	// channel uid
	UUID string
	Name string `json:"name,omitempty" groups:"api,config"`
	// channel source av stream
	URL    string `json:"url,omitempty" groups:"api,config"`
	source *AvStream
	// channel stream clients
	clients map[string]*ClientST
	// channel update. or codec update.
	// updated chan bool
	cond *sync.Cond

	// opreation signal.
	signals chan int
	///////////////////////////////////////////////////

	// auto streaming flag. FALSE==auto. default:false.
	OnDemand bool `json:"on_demand,omitempty" groups:"api,config"`
	Debug    bool `json:"debug,omitempty" groups:"api,config"`
	// HLS
	hlsSegmentBuffer map[int]*Segment
	hlsSegmentNumber int
}

//AvStream read data from source stream.
type AvStream struct {
	protocol int
	status   int
	avCodecs []av.CodecData
	sdp      []byte
	avQue    *pubsub.Queue
}

//ClientST client read avPkt from queue, write to chan.
type ClientST struct {
	UUID             string
	protocol         int
	signals          chan int
	outgoingAVPacket chan *av.Packet
}

//Segment HLS cache section
type Segment struct {
	dur  time.Duration
	data []*av.Packet
}
