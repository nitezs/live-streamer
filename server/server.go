package server

import (
	"embed"
	"html/template"
	"live-streamer/config"
	"live-streamer/streamer"
	mywebsocket "live-streamer/websocket"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	uuid "github.com/gofrs/uuid/v5"
	"github.com/gorilla/websocket"
)

//go:embed static
var staticFiles embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type InputFunc func(mywebsocket.RequestType)

type Server struct {
	addr          string
	dealInputFunc InputFunc
	clients       map[string]*Client
	mu            sync.Mutex
}

type Client struct {
	id          string
	conn        *websocket.Conn
	mu          sync.Mutex
	hasSentSize int
}

var GlobalServer *Server

func NewServer(addr string, dealInputFunc InputFunc) {
	GlobalServer = &Server{
		addr:          addr,
		dealInputFunc: dealInputFunc,
		clients:       make(map[string]*Client),
	}
}

func (s *Server) Run() {
	router := gin.New()
	tpl, err := template.ParseFS(staticFiles, "static/*")
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}
	router.SetHTMLTemplate(tpl)

	router.GET("/ws", AuthMiddleware(), s.handleWebSocket)
	router.GET(
		"/", func(c *gin.Context) {
			c.HTML(200, "index.html", nil)
		},
	)

	go func() {
		if err := router.Run(s.addr); err != nil {
			log.Fatalf("Error starting server: %v", err)
		}
	}()
}

func (s *Server) handleWebSocket(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	ws.SetCloseHandler(func(code int, text string) error {
		return nil
	})

	id, err := uuid.NewV7()
	if err != nil {
		log.Printf("generating uuid error: %v", err)
		return
	}
	client := &Client{id: id.String(), conn: ws, hasSentSize: 0}
	s.mu.Lock()
	s.clients[client.id] = client
	s.mu.Unlock()

	defer func() {
		client.mu.Lock()
		ws.Close()
		client.mu.Unlock()
		s.mu.Lock()
		delete(s.clients, client.id)
		s.mu.Unlock()
		if r := recover(); r != nil {
			log.Printf("webSocket handler panic: %v", r)
		}
	}()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			streamer.GlobalStreamer.TruncateOutput()
			currentVideoPath, _ := streamer.GlobalStreamer.GetCurrentVideoPath()
			s.Broadcast(mywebsocket.Date{
				Timestamp:        time.Now().UnixMilli(),
				CurrentVideoPath: currentVideoPath,
				VideoList:        streamer.GlobalStreamer.GetVideoListPath(),
				Output:           streamer.GlobalStreamer.GetOutput(),
			})
		}
	}()

	for {
		// recive message
		client.mu.Lock()
		msg := mywebsocket.Request{}
		err := ws.ReadJSON(&msg)
		client.mu.Unlock()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			break
		}
		s.dealInputFunc(msg.Type)
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.GlobalConfig.Auth.Token == "" ||
			c.Query("token") == config.GlobalConfig.Auth.Token {
			c.Next()
		} else {
			c.AbortWithStatus(http.StatusUnauthorized)
		}
	}
}

func (s *Server) Broadcast(obj mywebsocket.Date) {
	s.mu.Lock()
	for _, client := range s.clients {
		obj.Timestamp = time.Now().UnixMilli()
		if err := client.conn.WriteJSON(obj); err != nil {
			log.Printf("websocket writing message error: %v", err)
		}
	}
	s.mu.Unlock()
}

func (s *Server) Single(userID string, obj mywebsocket.Date) {
	s.mu.Lock()
	if client, ok := s.clients[userID]; ok {
		obj.Timestamp = time.Now().UnixMilli()
		if err := client.conn.WriteJSON(obj); err != nil {
			log.Printf("websocket writing message error: %v", err)
		}
	}
	s.mu.Unlock()
}

func (s *Server) Close() {

}
