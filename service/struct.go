package gss

import (
	"sync"
	"time"

	"github.com/cgoder/vdk/av"
	"github.com/cgoder/vdk/av/pubsub"
)

//player type
const (
	PLAY_MSE = iota
	PLAY_WEBRTC
	PLAY_HLS
	PLAY_HTTPFLV
	PLAY_RTSP
	PLAY_RTMP
)

//stream status type
const (
	STREAM_OFFLINE = iota
	STREAM_ONLINE
	STREAM_PAUSE
)

//signal type
const (
	SIGNAL_STREAM_UNKNOWN = iota
	SIGNAL_STREAM_STOP
	SIGNAL_STREAM_REFRESH
	SIGNAL_STREAM_AVCODEC_UPDATE
)

//StorageST main storage struct
type ServerST struct {
	mutex    sync.RWMutex
	Conf     ConfigST              `json:"conf" groups:"api,config"`
	Programs map[string]*ProgramST `json:"program" groups:"api,config"`
}

//ConfigST server storage section
type ConfigST struct {
	Debug        bool   `json:"debug" groups:"api,config"`
	LogLevel     string `json:"log_level" groups:"api,config"`
	HTTPDemo     bool   `json:"http_demo" groups:"api,config"`
	HTTPDebug    bool   `json:"http_debug" groups:"api,config"`
	HTTPLogin    string `json:"http_login" groups:"api,config"`
	HTTPPassword string `json:"http_password" groups:"api,config"`
	HTTPDir      string `json:"http_dir" groups:"api,config"`
	HTTPPort     string `json:"http_port" groups:"api,config"`
	RTSPPort     string `json:"rtsp_port" groups:"api,config"`
}

//ProgramST stream storage section
type ProgramST struct {
	UUID     string
	Name     string                `json:"name,omitempty" groups:"api,config"`
	Channels map[string]*ChannelST `json:"channels,omitempty" groups:"api,config"`
}

type ChannelST struct {
	// channel uid
	UUID string
	Name string `json:"name,omitempty" groups:"api,config"`
	// channel source stream url
	URL string `json:"url,omitempty" groups:"api,config"`
	// channel source stream
	source *AvStream
	// channel stream clients
	clients map[string]*ClientST
	// channel update. or codec update.
	cond *sync.Cond

	// opreation signal.
	signals chan int
	///////////////////////////////////////////////////

	// auto streaming flag. FALSE==auto. default:false.
	OnDemand bool `json:"on_demand,omitempty" groups:"api,config"`
	Debug    bool `json:"debug,omitempty" groups:"api,config"`
	///////////////////////////////////////////////////

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
	//ring buffer?
	avQue *pubsub.Queue
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
