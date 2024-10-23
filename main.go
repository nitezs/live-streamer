package main

import (
	"bufio"
	"fmt"
	"live-streamer/config"
	"live-streamer/server"
	"live-streamer/streamer"
	"live-streamer/utils"
	"log"
	"os"

	"github.com/fsnotify/fsnotify"
)

var GlobalStreamer *streamer.Streamer

func main() {
	server.NewServer(":8080", websocketRequestHandler)
	server.GlobalServer.Run()
	if !utils.HasFFMPEG() {
		log.Fatal("ffmpeg not found")
	}
	GlobalStreamer = streamer.NewStreamer(config.GlobalConfig.VideoList)
	go startWatcher()
	go input()
	GlobalStreamer.Stream()
	GlobalStreamer.Close()
}

func input() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text() // 获取用户输入的内容
		switch line {
		case "list":
			fmt.Println(GlobalStreamer.GetVideoListPath())
		case "index":
			fmt.Println(GlobalStreamer.GetCurrentIndex())
		case "next":
			GlobalStreamer.Next()
		case "prev":
			GlobalStreamer.Prev()
		case "quit":
			GlobalStreamer.Close()
			os.Exit(0)
		case "current":
			fmt.Println(GlobalStreamer.GetCurrentVideoPath())
		}
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
				}
			}
			if event.Op&fsnotify.Remove == fsnotify.Remove {
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
