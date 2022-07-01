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

type Sala struct {
	name   string
	users  [2]User
	images [24]string
}

type Cards struct {
	YourCard string
	Images   [24]string
}

var salas = make(map[string]*Sala)

func main() {
	port := flag.Int("p", 3669, "Port in which the updater will listen")
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

	http.Handle("/", http.HandlerFunc(generalHandlerFunc))      //http.FileServer(http.Dir("./"))
	http.Handle("/status", http.HandlerFunc(statusHandlerFunc)) //http.FileServer(http.Dir("./"))
	err = http.ListenAndServe(fmt.Sprint("localhost:", *port), nil)
	if err != nil {
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
	}
}

func generalHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		err := log.Output(1, fmt.Sprint("Method not allowed in generalHandlerFunc: ", r.Method))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		return
	}
	w.Header().Add("X-Frame-Options", "DENY")
	w.Header().Add("X-Content-Type-Options", "nosniff")
	w.Header().Add("Content-Security-Policy", "default-src 'self'")
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err != nil && err != http.ErrNoCookie {
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
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
	sala := r.URL.Query().Get("sala")
	delete := r.URL.Query().Get("delete")
	if delete == "yes" {
		deleteHandlerFunc(&w, r)
	}
	if sala != "" {
		salaHandlerFunc(&w, r, userCookie, sala)
	} else {
		http.FileServer(http.Dir("mento-mukatte-ui/")).ServeHTTP(w, r)
	}
}

func salaHandlerFunc(w *http.ResponseWriter, r *http.Request, usercookie *http.Cookie, nomeDaSala string) {
	sala, existe := salas[nomeDaSala]
	missingPlayer := 0
	alreadyUser := false
	cards := Cards{}
	userUUID, err := uuid.Parse(usercookie.Value)
	if err != nil {
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		(*w).WriteHeader(http.StatusBadRequest)
		return
	}
	if existe {
		i := 0
		for i < len(sala.users) {
			if sala.users[i].img == "" {
				missingPlayer = i
				break
			}
			i++
		}
		for i, v := range sala.users {
			if v.uuid == userUUID {
				alreadyUser = true
				missingPlayer = i
			}
		}
		if i == len(sala.users) && !alreadyUser {
			(*w).Write([]byte("Modo espectador ainda não implementado, peça para alguem compartilhar a tela."))
			return
		}
	} else {
		sala = &Sala{
			name:   nomeDaSala,
			users:  [2]User{},
			images: [24]string{},
		}
		imagens, err := os.ReadDir("./mento-mukatte-ui/img/")
		if err != nil || len(imagens) < len(sala.images) {
			err := log.Output(1, fmt.Sprint(err))
			if err != nil {
				log.Fatalf("COULD NOT WRITE TO LOG FILE")
			}
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		}
		for i := len(imagens) - 1; i > 0; i-- {
			j, err := rand.Int(rand.Reader, big.NewInt(int64(i)))
			if err != nil {
				err := log.Output(1, fmt.Sprint(err))
				if err != nil {
					log.Fatalf("COULD NOT WRITE TO LOG FILE")
				}
				(*w).WriteHeader(http.StatusInternalServerError)
				return
			}
			imagens[int(j.Int64())], imagens[i] = imagens[i], imagens[int(j.Int64())]
		}
		for i := range sala.images {
			sala.images[i] = imagens[i].Name()
		}
	}
	if alreadyUser {
		cards = Cards{
			Images:   sala.images,
			YourCard: sala.users[missingPlayer].img,
		}
	} else {
		i, err := rand.Int(rand.Reader, big.NewInt(int64(len(sala.images))))
		if err != nil {
			err := log.Output(1, fmt.Sprint(err))
			if err != nil {
				log.Fatalf("COULD NOT WRITE TO LOG FILE")
			}
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		}
		sala.users[missingPlayer].img = sala.images[int(i.Int64())]
		sala.users[missingPlayer].uuid = userUUID
		if err != nil {
			err := log.Output(1, fmt.Sprint(err))
			if err != nil {
				log.Fatalf("COULD NOT WRITE TO LOG FILE")
			}
			(*w).WriteHeader(http.StatusBadRequest)
			return
		}
		cards = Cards{
			Images:   sala.images,
			YourCard: sala.users[missingPlayer].img,
		}
	}
	salas[sala.name] = sala
	t, err := template.ParseFiles("mento-mukatte-ui/index.html")
	if err != nil {
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		(*w).WriteHeader(http.StatusInternalServerError)
		return
	}
	err = t.Execute((*w), cards)
	if err != nil {
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		(*w).WriteHeader(http.StatusInternalServerError)
		return
	}
}

func deleteHandlerFunc(w *http.ResponseWriter, r *http.Request) {
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err == http.ErrNoCookie {
		(*w).WriteHeader(http.StatusBadRequest)
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		return
	}
	if err != http.ErrNoCookie && err != nil {
		(*w).WriteHeader(http.StatusInternalServerError)
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		return
	}
	userUUID, err := uuid.Parse(userCookie.Value)
	if err != nil {
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		(*w).WriteHeader(http.StatusBadRequest)
		return
	}
	for _, sala := range salas {
		for _, v := range sala.users {
			if v.uuid == userUUID {
				delete(salas, sala.name)
				(*w).WriteHeader(http.StatusTemporaryRedirect)
				(*w).Header().Set("Location", fmt.Sprint("/sala=", sala.name))
				return
			}
		}
	}
	err = log.Output(1, "Usuario não está na sala e não pode ser deletado")
	if err != nil {
		log.Fatalf("COULD NOT WRITE TO LOG FILE")
	}
	(*w).WriteHeader(http.StatusForbidden)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func statusHandlerFunc(w http.ResponseWriter, r *http.Request) {
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err == http.ErrNoCookie {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No cookie"))
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		return
	}
	if err != http.ErrNoCookie && err != nil {
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	userUUID, err := uuid.Parse(userCookie.Value)
	if err != nil {
		err := log.Output(1, fmt.Sprint(err))
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	nomeDaSala := r.URL.Query().Get("sala")
	if nomeDaSala == "" {
		err := log.Output(1, "Não foi passado o nome da sala")
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sala, existe := salas[nomeDaSala]
	if !existe {
		err := log.Output(1, "A sala não existe")
		if err != nil {
			log.Fatalf("COULD NOT WRITE TO LOG FILE")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for i, v := range sala.users {
		if v.uuid == userUUID {
			upgrader.CheckOrigin = func(r *http.Request) bool { return true }
			c, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}
			//reader(c)
			var write chan []byte
			if sala.users[-i+1].dead == nil {
				write = make(chan []byte)
				sala.users[-i+1].dead = write
			} else {
				write = sala.users[-i+1].dead
			}
			var read chan []byte
			if sala.users[i].dead == nil {
				read = make(chan []byte)
				sala.users[i].dead = read
			} else {
				read = sala.users[i].dead
			}
			go writePump(c, write)
			go readPump(c, read)
		}
	}
	/*err = log.Output(1, "Usuario não está na sala")
	if err != nil {
		log.Fatalf("COULD NOT WRITE TO LOG FILE")
	}
	w.WriteHeader(http.StatusForbidden) */
}

func writePump(conn *websocket.Conn, write chan []byte) {
	for {
		dead := <-write
		log.Println("new message!!")
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

func readPump(conn *websocket.Conn, read chan []byte) {
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
