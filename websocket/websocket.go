package websocket

import (
	"live-streamer/streamer"
	"os"
)

type RequestType string

const (
	TypeStreamNextVideo RequestType = "StreamNextVideo"
	TypeStreamPrevVideo RequestType = "StreamPrevVideo"
	TypeQuit            RequestType = "Quit"
)

type Request struct {
	Type RequestType `json:"type"`
}

type Date struct {
	Timestamp        int64    `json:"timestamp"`
	CurrentVideoPath string   `json:"currentVideoPath"`
	VideoList        []string `json:"videoList"`
	Output           string   `json:"output"`
}

func RequestHandler(reqType RequestType) {
	switch reqType {
	case TypeStreamNextVideo:
		streamer.GlobalStreamer.Next()
	case TypeStreamPrevVideo:
		streamer.GlobalStreamer.Prev()
	case TypeQuit:
		streamer.GlobalStreamer.Close()
		os.Exit(0)
	}
}
