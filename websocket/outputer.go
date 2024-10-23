package websocket

type Outputer interface {
	Broadcast(v Response)
	Single(userID string, v Response)
}
