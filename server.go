package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type Game struct {
	Player1 *websocket.Conn
	Player2 *websocket.Conn
	Board   [3][3]rune
	lock    sync.Mutex
}

var games = make(map[string]*Game)
var lock = sync.Mutex{}
var gameID int

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func boardToString(board [3][3]rune) string {
	var str strings.Builder
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			str.WriteRune(board[i][j])
		}
		str.WriteString("\n")
	}
	return str.String()
}

func handleWebSocket(conn *websocket.Conn, game *Game) {
	var currentPlayer *websocket.Conn

	defer func() {
		conn.Close()
		lock.Lock()
		delete(games, strconv.Itoa(gameID))
		lock.Unlock()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Println("Cliente se ha desconectado.")
			} else {
				log.Println("Error al leer mensaje del cliente:", err)
			}
			break
		}
		fmt.Printf("Mensaje recibido del cliente: %s\n", message)

		parts := strings.Split(string(message), " ")
		if parts[0] == "JOIN" && parts[1] == "GAME" {
			lock.Lock()
			for id, g := range games {
				if g.Player2 == nil {
					game = g
					gameID, err = strconv.Atoi(id)
					if err != nil {
						log.Println("Error al convertir id a entero:", err)
						continue
					}
					game.Player2 = conn
					currentPlayer = game.Player1
					break
				}
			}
			if game == nil {
				gameID = len(games) + 1
				game = &Game{Player1: conn}
				games[strconv.Itoa(gameID)] = game
				currentPlayer = game.Player1
			}
			lock.Unlock()
		} else if len(parts) == 4 && parts[0] == "MOVE" {
			if conn != currentPlayer {
				conn.WriteMessage(websocket.TextMessage, []byte("ERROR: No es tu turno!"))
				continue
			}

			x, _ := strconv.Atoi(parts[2])
			y, _ := strconv.Atoi(parts[3])

			if x < 0 || x > 2 || y < 0 || y > 2 {
				conn.WriteMessage(websocket.TextMessage, []byte("ERROR: Movimiento Invalido!"))
				continue
			}

			game.lock.Lock()
			if game.Board[x][y] != 0 {
				conn.WriteMessage(websocket.TextMessage, []byte("ERROR: Ya esta ocupado!"))
				game.lock.Unlock()
				continue
			}

			var currentPlayerStr, loserPlayerStr string
			if conn == game.Player1 {
				game.Board[x][y] = 'X'
				currentPlayerStr = "Player 1"
			} else {
				game.Board[x][y] = 'O'
				currentPlayerStr = "Player 2"
			}
			game.lock.Unlock()

			if checkWinner(game.Board) {
				fmt.Println("Tenemos un ganador!")
				if conn == game.Player1 {
					game.Board[x][y] = 'X'
					currentPlayerStr = "PLAYER 1"
					loserPlayerStr = "PLAYER 2"
				} else {
					game.Board[x][y] = 'O'
					currentPlayerStr = "PLAYER 2"
					loserPlayerStr = "PLAYER 1"
				}
				winningMessage := fmt.Sprintf("GAME OVER %s, %s HA GANADO", loserPlayerStr, currentPlayerStr)
				game.Player1.WriteMessage(websocket.TextMessage, []byte(winningMessage))
				game.Player2.WriteMessage(websocket.TextMessage, []byte(winningMessage))
				gameOverMessage := "EL JUEGO HA TERMINADO"
				fmt.Println(gameOverMessage)
				game.Player1.WriteMessage(websocket.TextMessage, []byte(gameOverMessage))
				game.Player2.WriteMessage(websocket.TextMessage, []byte(gameOverMessage))
				return
			}

			if currentPlayer == game.Player1 {
				currentPlayer = game.Player2
			} else {
				currentPlayer = game.Player1
			}

			game.Player1.WriteMessage(websocket.TextMessage, []byte("UPDATE "+boardToString(game.Board)))
			game.Player2.WriteMessage(websocket.TextMessage, []byte("UPDATE "+boardToString(game.Board)))
		}
	}
}

func checkWinner(board [3][3]rune) bool {
	for i := 0; i < 3; i++ {
		if (board[i][0] != 0) && (board[i][0] == board[i][1]) && (board[i][1] == board[i][2]) {
			return true
		}
	}
	for i := 0; i < 3; i++ {
		if (board[0][i] != 0) && (board[0][i] == board[1][i]) && (board[1][i] == board[2][i]) {
			return true
		}
	}
	if (board[0][0] != 0) && (board[0][0] == board[1][1]) && (board[1][1] == board[2][2]) {
		return true
	}

	if (board[0][2] != 0) && (board[0][2] == board[1][1]) && (board[1][1] == board[2][0]) {
		return true
	}

	return false
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	conn, err := upgrader.Upgrade(w, r, nil)

	fmt.Println("Cliente conectado desde:", r.RemoteAddr)
	if err != nil {
		log.Println("Error al actualizar la conexión WebSocket:", err)
		return
	}

	lock.Lock()
	var game *Game
	for _, g := range games {
		if g.Player2 == nil {
			game = g
			break
		}
	}
	if game == nil {
		gameID++
		game = &Game{Player1: conn}
		games[strconv.Itoa(gameID)] = game
		conn.WriteMessage(websocket.TextMessage, []byte("PLAYER 1"))
	} else {
		game.Player2 = conn
		conn.WriteMessage(websocket.TextMessage, []byte("PLAYER 2"))
	}
	lock.Unlock()

	go handleWebSocket(conn, game)
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	fmt.Println("Servidor WebSocket escuchando en ws://localhost:52301/ws y en tu dirección IP pública")

	err := http.ListenAndServe(":52301", nil)
	if err != nil {
		log.Fatal("Error al iniciar el servidor:", err)
	}
}
