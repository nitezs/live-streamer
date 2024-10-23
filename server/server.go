package server

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

//go:embed static
var staticFiles embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type InputFunc func(string)

type Server struct {
	addr          string
	outputChan    chan string
	dealInputFunc InputFunc
	clients       []*Client
	historyOutput string
}

type Client struct {
	conn *websocket.Conn
}

var GlobalServer *Server

func NewServer(addr string, dealInputFunc InputFunc) {
	GlobalServer = &Server{
		addr:          addr,
		outputChan:    make(chan string),
		dealInputFunc: dealInputFunc,
	}
}

func (s *Server) Run() {
	router := gin.Default()
	tpl, err := template.ParseFS(staticFiles, "static/*")
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}
	router.SetHTMLTemplate(tpl)

	router.GET("/ws", s.handleWebSocket)
	router.GET("/video/current", GetCurrentVideo)
	router.GET("/video/list", GetVideoList)
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

	go func() {
		for {
			output := <-s.outputChan
			s.historyOutput += output
			for _, client := range s.clients {
				_ = client.conn.WriteMessage(websocket.TextMessage, []byte(output))
			}
		}
	}()
}

func (s *Server) handleWebSocket(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	client := &Client{conn: ws}
	s.clients = append(s.clients, client)
	_ = client.conn.WriteMessage(websocket.TextMessage, []byte(s.historyOutput))

	defer func() {
		for i, c := range s.clients {
			if c == client {
				s.clients = append(s.clients[:i], s.clients[i+1:]...)
				break
			}
		}
	}()

	for {
		// recive message
		_, msg, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Websocket reading message error: %v", err)
			break
		}
		s.dealInputFunc(string(msg))
	}
}

func (s *Server) Print(msg ...any) {
	s.outputChan <- fmt.Sprint(msg...)
}

func (s *Server) Println(msg ...any) {
	s.outputChan <- fmt.Sprintln(msg...)
}

func (s *Server) Printf(format string, args ...interface{}) {
	s.outputChan <- fmt.Sprintf(format, args...)
}

func (s *Server) Close() {
	close(s.outputChan)
}
