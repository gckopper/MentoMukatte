package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type User struct {
	img  string
	uuid uuid.UUID
	dead chan []byte
}

type Room struct {
	name   string
	users  [2]User
	images [24]string
}

type Cards struct {
	YourCard string
	Images   [24]string
}

var rooms = make(map[string]Room)

func main() {
	port := flag.Int("p", 3669, "Port in which the game server will listen")
	flag.Parse()

	// open a file
	logFile, err := os.OpenFile(".log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		fmt.Printf("error opening file: %v", err)
	}

	// don't forget to close it
	defer func(logFile *os.File) {
		err = logFile.Close()
		if err != nil {
			log.Fatalf("COULD NOT OPEN LOG FILE")
		}
	}(logFile)

	// assign it to the standard logger
	log.SetOutput(logFile)

	http.Handle("/", http.HandlerFunc(generalHandlerFunc)) //http.FileServer(http.Dir("./"))
	err = http.ListenAndServe(fmt.Sprint("localhost:", *port), nil)
	if err != nil {
		log.Println(err)
	}
}

func generalHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		log.Println("Method not allowed in generalHandlerFunc: ", r.Method)
		return
	}
	w.Header().Add("X-Frame-Options", "DENY")
	w.Header().Add("X-Content-Type-Options", "nosniff")
	w.Header().Add("Content-Security-Policy", "default-src 'self'; script-src 'self'")
	w.Header().Add("Strict-Transport-Security", "max-age=63072000;")
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err != nil && err != http.ErrNoCookie {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err == http.ErrNoCookie {
		newCookie := http.Cookie{
			HttpOnly: true,
			Secure:   true,
			Name:     "SessionCookie",
			Value:    uuid.New().String(),
			MaxAge:   31536000,
			Expires:  time.Now().Add(time.Hour * 24 * 365), // Give it a year of life
			SameSite: http.SameSiteStrictMode,              // Set SameSite to strict as a way of mitigating attacks
		}
		http.SetCookie(w, &newCookie)
		userCookie = &newCookie
	}
	roomName := r.URL.Query().Get("sala")
	if r.URL.Path == "/status" {
		statusHandlerFunc(&w, r, userCookie, roomName)
		return
	}
	delete := r.URL.Query().Get("delete")
	if delete == "yes" {
		deleteHandlerFunc(&w, r, userCookie)
		return
	}
	if roomName != "" {
		roomHandlerFunc(&w, r, userCookie, roomName)
		return
	}
	http.FileServer(http.Dir("mento-mukatte-ui/")).ServeHTTP(w, r)

}

func roomHandlerFunc(w *http.ResponseWriter, r *http.Request, usercookie *http.Cookie, roomName string) {
	room, exists := rooms[roomName]
	missingPlayer := 0
	alreadyUser := false
	cards := Cards{}
	userUUID, err := uuid.Parse(usercookie.Value)
	if err != nil {
		log.Println(err)
		(*w).WriteHeader(http.StatusBadRequest)
		return
	}
	if exists {
		i := 0
		for i < len(room.users) {
			if room.users[i].img == "" {
				missingPlayer = i
				break
			}
			i++
		}
		for i, v := range room.users {
			if v.uuid == userUUID {
				alreadyUser = true
				missingPlayer = i
			}
		}
		if i == len(room.users) && !alreadyUser {
			(*w).Write([]byte("Modo espectador ainda não implementado, peça para alguem compartilhar a tela."))
			return
		}
	} else {
		room = Room{
			name:   roomName,
			users:  [2]User{},
			images: [24]string{},
		}
		images, err := os.ReadDir("./mento-mukatte-ui/img/")
		if err != nil || len(images) < len(room.images) {
			log.Println(err)
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		}
		for i := len(images) - 1; i > 0; i-- {
			j, err := rand.Int(rand.Reader, big.NewInt(int64(i)))
			if err != nil {
				log.Println(err)
				(*w).WriteHeader(http.StatusInternalServerError)
				return
			}
			images[int(j.Int64())], images[i] = images[i], images[int(j.Int64())]
		}
		for i := range room.images {
			room.images[i] = images[i].Name()
		}
	}
	if alreadyUser {
		cards = Cards{
			Images:   room.images,
			YourCard: room.users[missingPlayer].img,
		}
	} else {
		i, err := rand.Int(rand.Reader, big.NewInt(int64(len(room.images))))
		if err != nil {
			log.Println(err)
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		}
		room.users[missingPlayer].img = room.images[int(i.Int64())]
		room.users[missingPlayer].uuid = userUUID
		if err != nil {
			log.Println(err)
			(*w).WriteHeader(http.StatusBadRequest)
			return
		}
		cards = Cards{
			Images:   room.images,
			YourCard: room.users[missingPlayer].img,
		}
	}
	t, err := template.ParseFiles("mento-mukatte-ui/index.html")
	if err != nil {
		log.Println(err)
		(*w).WriteHeader(http.StatusInternalServerError)
		return
	}
	err = t.Execute((*w), cards)
	if err != nil {
		log.Println(err)
		(*w).WriteHeader(http.StatusInternalServerError)
		return
	}
	rooms[room.name] = room

}

func deleteHandlerFunc(w *http.ResponseWriter, r *http.Request, userCookie *http.Cookie) {
	userUUID, err := uuid.Parse(userCookie.Value)
	if err != nil {
		log.Println(err)
		(*w).WriteHeader(http.StatusBadRequest)
		return
	}
	for _, room := range rooms {
		for _, v := range room.users {
			if v.uuid == userUUID {
				delete(rooms, room.name)
				(*w).WriteHeader(http.StatusOK)
				return
			}
		}
	}
	log.Println("Usuario não está na sala e não pode ser deletado")
	(*w).WriteHeader(http.StatusForbidden)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func statusHandlerFunc(w *http.ResponseWriter, r *http.Request, userCookie *http.Cookie, roomName string) {
	userUUID, err := uuid.Parse(userCookie.Value)
	if err != nil {
		log.Println(err)
		(*w).WriteHeader(http.StatusBadRequest)
		return
	}
	room, exists := rooms[roomName]
	if !exists {
		log.Println("A sala não existe")
		(*w).WriteHeader(http.StatusBadRequest)
		return
	}
	for i, v := range room.users {
		if v.uuid == userUUID {
			upgrader.CheckOrigin = func(r *http.Request) bool { return true }
			c, err := upgrader.Upgrade(*w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}
			// Write channel
			var write chan []byte
			if room.users[-i+1].dead == nil {
				write = make(chan []byte)
				room.users[-i+1].dead = write
			} else {
				write = room.users[-i+1].dead
			}
			// Read channel
			var read chan []byte
			if room.users[i].dead == nil {
				read = make(chan []byte)
				room.users[i].dead = read
			} else {
				read = room.users[i].dead
			}
			go writePump(c, write)
			go readPump(c, read)
		}
	}
}

func writePump(conn *websocket.Conn, write <-chan []byte) {
	for {
		dead := <-write
		w, err := conn.NextWriter(websocket.TextMessage)
		if err != nil {
			log.Println(err)
			return
		}
		w.Write([]byte(base64.StdEncoding.EncodeToString(dead)))
		if err := w.Close(); err != nil {
			log.Println(err)
			return
		}
	}
}

func readPump(conn *websocket.Conn, read chan<- []byte) {
	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		if mt == websocket.TextMessage {
			b, err := base64.StdEncoding.DecodeString(string(message))
			if err != nil {
				log.Println(err)
				return
			}
			read <- b
		}
	}
}
