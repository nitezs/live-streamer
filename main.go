package main

import (
	"live-streamer/config"
	"live-streamer/logger"
	"live-streamer/server"
	"live-streamer/streamer"
	"live-streamer/utils"
	"log"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
)

var GlobalStreamer *streamer.Streamer
var Logger logger.Logger

func main() {
	server.NewServer(":8080", input)
	server.GlobalServer.Run()
	Logger = server.GlobalServer
	if !utils.HasFFMPEG() {
		log.Fatal("ffmpeg not found")
	}
	GlobalStreamer = streamer.NewStreamer(config.GlobalConfig.VideoList, Logger)
	go startWatcher()
	GlobalStreamer.Stream()
	GlobalStreamer.Close()
}

func input(msg string) {
	switch msg {
	case "prev":
		GlobalStreamer.Prev()
	case "next":
		GlobalStreamer.Next()
	case "quit":
		GlobalStreamer.Close()
		os.Exit(0)
	case "list":
		list := GlobalStreamer.GetVideoListPath()
		Logger.Println("\nvideo list:\n", strings.Join(list, "\n"))
	case "current":
		videoPath, err := GlobalStreamer.GetCurrentVideoPath()
		if err != nil {
			Logger.Println("current video: none")
		}
		Logger.Println("current video: ", videoPath)
	default:
		Logger.Println("unknown command")
	}
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
			Logger.Println("watching dir:", item.Path)
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
					Logger.Println("new video added:", event.Name)
					GlobalStreamer.Add(event.Name)
				}
			}
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				Logger.Println("video removed:", event.Name)
				GlobalStreamer.Remove(event.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			Logger.Println("watcher error:", err)
		}
	}
}
