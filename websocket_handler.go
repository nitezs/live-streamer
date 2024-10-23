package main

import (
	"live-streamer/websocket"
	"os"
)

func websocketRequestHandler(reqType websocket.RequestType) {
	switch reqType {
	case websocket.TypeStreamNextVideo:
		GlobalStreamer.Next()
	case websocket.TypeStreamPrevVideo:
		GlobalStreamer.Prev()
	case websocket.TypeQuit:
		GlobalStreamer.Close()
		os.Exit(0)
	}
}
