package streamer

import (
	"bufio"
	"fmt"
	"io"
	"live-streamer/config"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Streamer struct {
	playlist          []config.InputItem
	currentVideoIndex int
	cmd               *exec.Cmd
	stopped           bool
	logFile           *os.File
	doneCh            chan bool
	mu                sync.Mutex
	manualNext        bool
}

func NewStreamer(playList []config.InputItem) *Streamer {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Error creating log directory: %v\n", err)
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("ffmpeg_%s.log", time.Now().Format("2006-01-02_15-04-05")))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening log file: %v\n", err)
	}
	return &Streamer{
		playlist:          playList,
		currentVideoIndex: 0,
		cmd:               nil,
		logFile:           logFile,
		doneCh:            make(chan bool, 1),
	}
}

func (s *Streamer) Add(videoPath string) {
	s.playlist = append(s.playlist, config.InputItem{Path: videoPath})
}

func (s *Streamer) Remove(videoPath string) {
	for i, item := range s.playlist {
		if item.Path == videoPath {
			s.playlist = append(s.playlist[:i], s.playlist[i+1:]...)
			if s.currentVideoIndex >= len(s.playlist) {
				s.currentVideoIndex = 0
			}
			if s.currentVideoIndex == i {
				s.start()
			}
			break
		}
	}
}

func (s *Streamer) Prev() {
	s.currentVideoIndex--
	if s.currentVideoIndex < 0 {
		s.currentVideoIndex = len(s.playlist) - 1
	}
	s.start()
}

func (s *Streamer) Next() {
	s.manualNext = true
	s.currentVideoIndex++
	if s.currentVideoIndex >= len(s.playlist) {
		s.currentVideoIndex = 0
	}
	s.start()
}

func (s *Streamer) Stream() {
	for {
		if len(s.playlist) == 0 {
			time.Sleep(time.Second)
			continue
		}
		if s.currentVideoIndex >= len(s.playlist) {
			s.currentVideoIndex = 0
		}
		s.start()
	}
}

func (s *Streamer) buildFFmpegArgs(videoItem config.InputItem) []string {
	videoPath := videoItem.Path

	args := []string{"-re"}
	if videoItem.Start != "" {
		args = append(args, "-ss", videoItem.Start)
	}

	args = append(args, "-i", videoPath)

	if videoItem.End != "" {
		args = append(args, "-to", videoItem.End)
	}

	args = append(args,
		"-c:v", config.GlobalConfig.Play.VideoCodec,
		"-preset", config.GlobalConfig.Play.Preset,
		"-crf", fmt.Sprintf("%d", config.GlobalConfig.Play.CRF),
		"-maxrate", config.GlobalConfig.Play.MaxRate,
		"-bufsize", config.GlobalConfig.Play.BufSize,
		"-vf", fmt.Sprintf("scale=%s", config.GlobalConfig.Play.Scale),
		"-r", fmt.Sprintf("%d", config.GlobalConfig.Play.FrameRate),
		"-c:a", config.GlobalConfig.Play.AudioCodec,
		"-b:a", config.GlobalConfig.Play.AudioBitrate,
		"-ar", fmt.Sprintf("%d", config.GlobalConfig.Play.AudioSampleRate),
		"-f", config.GlobalConfig.Play.OutputFormat,
		"-stats", "-loglevel", "info",
	)

	if config.GlobalConfig.Play.CustomArgs != "" {
		customArgs := strings.Fields(config.GlobalConfig.Play.CustomArgs)
		args = append(args, customArgs...)
	}

	args = append(args, fmt.Sprintf("%s/%s", config.GlobalConfig.Output.RTMPServer, config.GlobalConfig.Output.StreamKey))

	// log.Println("ffmpeg args: ", args)

	return args
}

func (s *Streamer) start() {
	defer func() {
		s.cmd = nil
	}()
	if s.cmd != nil && !s.stopped {
		s.Stop()
	}
	s.mu.Lock()
	s.stopped = false
	s.mu.Unlock()

	currentVideo := s.playlist[s.currentVideoIndex]
	videoPath := currentVideo.Path
	log.Println("start stream: ", videoPath)

	s.cmd = exec.Command("ffmpeg", s.buildFFmpegArgs(currentVideo)...)

	pipe, err := s.cmd.StderrPipe()
	if err != nil {
		log.Printf("failed to get pipe: %v", err)
		return
	}

	reader := bufio.NewReader(pipe)
	writer := bufio.NewWriter(s.logFile)

	if err := s.cmd.Start(); err != nil {
		log.Printf("starting ffmpeg error: %v\n", err)
		return
	}

	go func() {
		defer func() {
			if s.logFile != nil {
				if err := s.logFile.Sync(); err != nil {
					log.Printf("syncing log file error: %v\n", err)
				}
			}
		}()
		buf := make([]byte, 1024)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF && !s.stopped {
					log.Printf("reading ffmpeg error: %v\n", err)
				}
				break
			}
			if n > 0 {
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				logLine := fmt.Sprintf("[%s] %s", timestamp, string(buf[:n]))
				if s.logFile != nil {
					if _, err := writer.WriteString(logLine); err != nil {
						log.Printf("writing to log file error: %v\n", err)
					}
					if err := writer.Flush(); err != nil {
						log.Printf("flushing writer error: %v\n", err)
					}
				}
			}
		}
	}()

	go func() {
		err = s.cmd.Wait()
		s.doneCh <- true
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil && !s.stopped {
			log.Printf("ffmpeg streaming error: %v\nStart streaming next video\n", err)
			s.Next()
		} else {
			log.Println("ffmpeg streaming stopped")
			if !s.manualNext {
				s.Next()
			}
		}
	}()

	<-s.doneCh
}

func (s *Streamer) Stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.stopped = true
		_ = s.cmd.Process.Kill()
		<-s.doneCh
		s.cmd = nil
	}
}

func (s *Streamer) GetCurrentVideo() string {
	return s.playlist[s.currentVideoIndex].Path
}

func (s *Streamer) GetPlaylist() []config.InputItem {
	return s.playlist
}

func (s *Streamer) Close() {
	if s.logFile != nil {
		s.logFile.Close()
		s.logFile = nil
	}
}
