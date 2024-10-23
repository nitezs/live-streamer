package websocket

type MessageType string

var (
	TypeOutput              MessageType = "Output"
	TypeStreamNextVideo     MessageType = "StreamNextVideo"
	TypeStreamPrevVideo     MessageType = "StreamPrevVideo"
	TypeGetCurrentVideoPath MessageType = "GetCurrentVideoPath"
	TypeGetVideoList        MessageType = "GetVideoList"
	TypeQuit                MessageType = "Quit"
	TypeRemoveVideo         MessageType = "RemoveVideo"
	TypeAddVideo            MessageType = "AddVideo"
)

type Request struct {
	Type      MessageType `json:"type"`
	Args      []string    `json:"args"`
	UserID    string      `json:"user_id"`
	Timestamp int64       `json:"timestamp"`
}

type Response struct {
	Type      MessageType `json:"type"`
	Success   bool        `json:"success"`
	Data      any         `json:"data"`
	Message   string      `json:"message"`
	UserID    string      `json:"user_id"`
	Timestamp int64       `json:"timestamp"`
}

func MakeResponse(messageType MessageType, success bool, data any, message string) Response {
	return Response{
		Type:    messageType,
		Success: success,
		Data:    data,
		Message: message,
	}
}

func MakeOutput(output string) Response {
	return Response{
		Success: true,
		Type:    TypeOutput,
		Data:    output,
	}
}
