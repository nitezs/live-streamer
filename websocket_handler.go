package main

import (
	"live-streamer/server"
	"live-streamer/websocket"
)

func websocketRequestHandler(req websocket.Response) {
	if req.UserID == "" {
		return
	}
	var resp websocket.Response
	switch websocket.MessageType(req.Type) {
	case websocket.TypeStreamNextVideo:
		GlobalStreamer.Next()
		resp = websocket.Response{
			Type:    websocket.TypeStreamNextVideo,
			Success: true,
		}
		server.GlobalServer.Broadcast(resp)
	case websocket.TypeStreamPrevVideo:
		GlobalStreamer.Prev()
		resp = websocket.Response{
			Type:    websocket.TypeStreamPrevVideo,
			Success: true,
		}
		server.GlobalServer.Broadcast(resp)
	case websocket.TypeGetCurrentVideoPath:
		videoPath, err := GlobalStreamer.GetCurrentVideoPath()
		if err != nil {
			resp = websocket.Response{
				Type:    websocket.TypeGetCurrentVideoPath,
				Success: false,
				Message: err.Error(),
			}
		} else {
			resp = websocket.Response{
				Type:    websocket.TypeGetCurrentVideoPath,
				Success: true,
				Data:    videoPath,
			}
		}
		server.GlobalServer.Single(req.UserID, resp)
	case websocket.TypeGetVideoList:
		resp = websocket.Response{
			Type:    websocket.TypeGetVideoList,
			Success: true,
			Data:    GlobalStreamer.GetVideoListPath(),
		}
		server.GlobalServer.Single(req.UserID, resp)
	case websocket.TypeQuit:
		server.GlobalServer.Close()
		GlobalStreamer.Close()
		resp = websocket.Response{
			Type:    websocket.TypeQuit,
			Success: true,
		}
		server.GlobalServer.Broadcast(resp)
	}
}
