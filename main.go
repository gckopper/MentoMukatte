package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type User struct {
	img  string
	uuid uuid.UUID
	dead int
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

	http.Handle("/", http.HandlerFunc(generalHandlerFunc))      //http.FileServer(http.Dir("./"))
	http.Handle("/status", http.HandlerFunc(statusHandlerFunc)) //http.FileServer(http.Dir("./"))
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
	w.Header().Add("Content-Security-Policy", "default-src 'self'")
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err != nil && err != http.ErrNoCookie {
		log.Println(err)
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
	delete := r.URL.Query().Get("delete")
	if delete == "yes" {
		deleteHandlerFunc(&w, r)
	}
	if roomName != "" {
		roomHandlerFunc(&w, r, userCookie, roomName)
	} else {
		http.FileServer(http.Dir("mento-mukatte-ui/")).ServeHTTP(w, r)
	}
}

func roomHandlerFunc(w *http.ResponseWriter, r *http.Request, usercookie *http.Cookie, roomName string) {
	room, existe := rooms[roomName]
	missingPlayer := 0
	alreadyUser := false
	cards := Cards{}
	userUUID, err := uuid.Parse(usercookie.Value)
	if err != nil {
		log.Println(err)
		(*w).WriteHeader(http.StatusBadRequest)
		return
	}
	if existe {
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
		imagens, err := os.ReadDir("./Cara-a-cara/img/")
		if err != nil || len(imagens) < len(room.images) {
			log.Println(err)
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		}
		for i := len(imagens) - 1; i > 0; i-- {
			j, err := rand.Int(rand.Reader, big.NewInt(int64(i)))
			if err != nil {
				log.Println(err)
				(*w).WriteHeader(http.StatusInternalServerError)
				return
			}
			imagens[int(j.Int64())], imagens[i] = imagens[i], imagens[int(j.Int64())]
		}
		for i := range room.images {
			room.images[i] = imagens[i].Name()
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

func deleteHandlerFunc(w *http.ResponseWriter, r *http.Request) {
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err == http.ErrNoCookie {
		log.Println(err)
		(*w).WriteHeader(http.StatusBadRequest)
		return
	}
	if err != http.ErrNoCookie && err != nil {
		log.Println(err)
		(*w).WriteHeader(http.StatusInternalServerError)
		return
	}
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
				(*w).WriteHeader(http.StatusTemporaryRedirect)
				(*w).Header().Set("Location", fmt.Sprint("/sala=", room.name))
				return
			}
		}
	}
	log.Println("Usuario não está na sala e não pode ser deletado")
	(*w).WriteHeader(http.StatusForbidden)
}

func statusHandlerFunc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Println("Method not allowed in statusHandlerFunc: ", r.Method)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Add("X-Frame-Options", "DENY")
	w.Header().Add("Content-Security-Policy", "default-src 'self'")
	w.Header().Add("X-Content-Type-Options", "nosniff")
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err == http.ErrNoCookie {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No cookie"))
		log.Println(err)
		return
	}
	if err != http.ErrNoCookie && err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	userUUID, err := uuid.Parse(userCookie.Value)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	roomName := r.URL.Query().Get("sala")
	if roomName == "" {
		log.Println("Não foi passado o nome da sala")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	room, existe := rooms[roomName]
	if !existe {
		log.Println("A sala não existe")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for i, v := range room.users {
		if v.uuid == userUUID {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			room.users[i].dead, err = strconv.Atoi(string(body))
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if i == 0 {
				w.Write([]byte(fmt.Sprint(room.users[1].dead)))
			} else {
				w.Write([]byte(fmt.Sprint(room.users[0].dead)))
			}
			rooms[room.name] = room
			return
		}
	}
	log.Println("Usuario não está na sala")
	w.WriteHeader(http.StatusForbidden)
}
