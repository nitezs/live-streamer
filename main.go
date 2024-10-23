package main

import (
	"live-streamer/config"
	"live-streamer/server"
	"live-streamer/streamer"
	"live-streamer/utils"
	"live-streamer/websocket"
	"log"

	"github.com/fsnotify/fsnotify"
)

var GlobalStreamer *streamer.Streamer
var outputer websocket.Outputer

func main() {
	server.NewServer(":8080", websocketRequestHandler)
	server.GlobalServer.Run()
	outputer = server.GlobalServer
	if !utils.HasFFMPEG() {
		log.Fatal("ffmpeg not found")
	}
	GlobalStreamer = streamer.NewStreamer(config.GlobalConfig.VideoList, outputer)
	go startWatcher()
	GlobalStreamer.Stream()
	GlobalStreamer.Close()
}

func startWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()
	for _, item := range config.GlobalConfig.InputItems {
		if item.ItemType == "dir" {
			err = watcher.Add(item.Path)
			if err != nil {
				log.Fatalf("failed to add dir to watcher: %v", err)
			}
			log.Println("watching dir:", item.Path)
		}
	}
	if err != nil {
		log.Fatalf("failed to start watcher: %v", err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				if utils.IsSupportedVideo(event.Name) {
					log.Println("new video added:", event.Name)
					GlobalStreamer.Add(event.Name)
					server.GlobalServer.Broadcast(websocket.MakeResponse(websocket.TypeAddVideo, true, event.Name, ""))
				}
			}
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				server.GlobalServer.Broadcast(websocket.MakeResponse(websocket.TypeRemoveVideo, true, event.Name, ""))
				log.Println("video removed:", event.Name)
				GlobalStreamer.Remove(event.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("watcher error:", err)
		}
	}
}
