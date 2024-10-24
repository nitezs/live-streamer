# Liver streamer

一个基于 Go 语言的自动视频推流工具。

## 功能特点

- 🎥 支持自动循环推流指定文件夹中的视频文件
- 🎮 提供 Web 控制面板实时监控推流状态
- ⚙️ 灵活的视频编码和推流参数配置
- 🎯 支持视频片段截取推流（指定开始和结束时间）
- 🔄 支持手动切换当前推流视频

## 示例配置

除了 input 和 output 部分，其余都是可选的

```json
{
  "input": [
    "./videos",
    {
      "path": "./video1.mp4",
      "start": "00:01:00",
      "end": "01:00:00"
    },
    {
      "path": "./video2.mkv",
      "start": "10s",
      "end": "100s"
    }
  ],
  "play": {
    "video_codec": "libx264",
    "preset": "medium",
    "crf": 23,
    "max_rate": "1000k",
    "buf_size": "2000k",
    "scale": "1920:1080",
    "frame_rate": 30,
    "audio_codec": "aac",
    "audio_bitrate": "128k",
    "audio_sample_rate": 44100,
    "output_format": "flv",
    "custom_args": ""
  },
  "output": {
    "rtmp_server": "rtmp://live-push.example.com/live",
    "stream_key": "your-stream-key"
  },
  "log": {
    "play_state": true
  },
  "server": {
    "addr": ":8080",
    "token": "your-access-token"
  }
}
```
