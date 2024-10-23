package websocket

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
