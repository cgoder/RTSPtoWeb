package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/gops/agent"
)

//Default streams signals
const (
	SignalStreamRestart = iota ///< Y   Restart
	SignalStreamStop
	SignalStreamCodecUpdate
	SignalStreamClient
)

//GenerateUUID function make random uuid for clients and stream
func GenerateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

//stringToInt convert string to int if err to zero
func stringToInt(val string) int {
	i, err := strconv.Atoi(val)
	if err != nil {
		return 0
	}
	return i
}

//stringInBetween fin char to char sub string
func stringInBetween(str string, start string, end string) (result string) {
	s := strings.Index(str, start)
	if s == -1 {
		return
	}
	str = str[s+len(start):]
	e := strings.Index(str, end)
	if e == -1 {
		return
	}
	str = str[:e]
	return str
}

//JsonFormat Json outupt.
func JsonFormat(v interface{}) string {
	// if msg == "" {
	// 	return ""
	// }
	var out bytes.Buffer

	bs, _ := json.Marshal(v)
	json.Indent(&out, bs, "", "\t")

	return out.String()
}

func DebugRuntime() {
	if err := agent.Listen(agent.Options{
		Addr:            "0.0.0.0:8048",
		ShutdownCleanup: true, // automatically closes on os.Interrupt
	}); err != nil {
		log.Fatal(err)
	}
	time.Sleep(time.Hour)
}
