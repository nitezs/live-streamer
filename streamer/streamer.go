package streamer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"live-streamer/config"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	CONTROL_ADD = iota
	CONTROL_REMOVE
	CONTROL_NEXT
	CONTROL_PREV
)

type streamerControl struct {
	cmd  int
	args []string
}

type Streamer struct {
	videoList         []config.InputItem
	currentVideoIndex int
	cmd               *exec.Cmd
	logFile           *os.File
	ctx               context.Context
	cancel            context.CancelFunc
	control           chan streamerControl
}

func NewStreamer(videoList []config.InputItem) *Streamer {
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
		videoList:         videoList,
		currentVideoIndex: 0,
		cmd:               nil,
		logFile:           logFile,
		ctx:               nil,
		control:           make(chan streamerControl),
	}
}

func (s *Streamer) Add(videoPath string) {
	s.control <- streamerControl{cmd: CONTROL_ADD, args: []string{videoPath}}
}

func (s *Streamer) Remove(videoPath string) {
	s.control <- streamerControl{cmd: CONTROL_REMOVE, args: []string{videoPath}}
}

func (s *Streamer) Prev() {
	s.control <- streamerControl{cmd: CONTROL_PREV}
}

func (s *Streamer) Next() {
	s.control <- streamerControl{cmd: CONTROL_NEXT}
}

func (s *Streamer) handleControl(req streamerControl) {
	switch req.cmd {
	case CONTROL_ADD:
		s.doAdd(req.args[0])
	case CONTROL_REMOVE:
		s.doRemove(req.args[0])
	case CONTROL_NEXT:
		s.doNext()
	case CONTROL_PREV:
		s.doPrev()
	}
}

func (s *Streamer) doAdd(videoPath string) {
	s.videoList = append(s.videoList, config.InputItem{Path: videoPath})
}

func (s *Streamer) doRemove(videoPath string) {
	for i, item := range s.videoList {
		if item.Path == videoPath {
			s.videoList = append(s.videoList[:i], s.videoList[i+1:]...)
			if s.currentVideoIndex >= len(s.videoList) {
				s.currentVideoIndex = 0
			}
			if s.currentVideoIndex == i {
				s.start()
			}
			break
		}
	}
}

func (s *Streamer) doPrev() {
	s.currentVideoIndex--
	if s.currentVideoIndex < 0 {
		s.currentVideoIndex = len(s.videoList) - 1
	}
	s.start()
}

func (s *Streamer) doNext() {
	s.currentVideoIndex++
	if s.currentVideoIndex >= len(s.videoList) {
		s.currentVideoIndex = 0
	}
	s.start()
}

func (s *Streamer) Stream() {
	for {
		if len(s.videoList) == 0 {
			time.Sleep(time.Second)
			continue
		}
		if s.currentVideoIndex >= len(s.videoList) {
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
	s.Stop()

	s.ctx, s.cancel = context.WithCancel(context.Background())

	currentVideo := s.videoList[s.currentVideoIndex]
	videoPath := currentVideo.Path
	log.Println("start stream: ", videoPath)

	args := s.buildFFmpegArgs(currentVideo)
	log.Printf("ffmpeg args: %v", args)

	s.cmd = exec.CommandContext(s.ctx, "ffmpeg", args...)

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

	go s.log(reader, writer)

	select {
	case req := <-s.control:
		s.handleControl(req)
	case <-s.ctx.Done():
		log.Println("case <-s.ctx.Done")
		if s.cmd != nil && s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
			s.cmd = nil
		}
	case err := <-waitCmd(s.cmd):
		log.Println("case err := <-waitCmd(s.cmd)")
		if err != nil {
			log.Printf("ffmpeg exited with error: %v", err)
		}
		s.doNext()
	}
}

func (s *Streamer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Streamer) GetCurrentVideo() string {
	return s.videoList[s.currentVideoIndex].Path
}

func (s *Streamer) GetPlaylist() []config.InputItem {
	return s.videoList
}

func (s *Streamer) Close() {
	if s.logFile != nil {
		s.logFile.Close()
		s.logFile = nil
	}
}

func waitCmd(cmd *exec.Cmd) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- cmd.Wait()
	}()
	return ch
}

func (s *Streamer) log(reader *bufio.Reader, writer *bufio.Writer) {
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
			if err != io.EOF {
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
}
