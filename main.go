package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

type User struct {
	img  string
	uuid uuid.UUID
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

var salas = make(map[string]Sala)

func main() {
	println("Hello, World!")
	http.Handle("/", http.HandlerFunc(generalHandlerFunc)) //http.FileServer(http.Dir("./"))
	err := http.ListenAndServe(fmt.Sprint("localhost:", 25565), nil)
	if err != nil {
		err = log.Output(0, fmt.Sprintln(err))
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func generalHandlerFunc(w http.ResponseWriter, r *http.Request) {
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err == http.ErrNoCookie {
		newCookie := http.Cookie{
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
	if sala != "" {
		salaHandlerFunc(&w, r, userCookie, sala)
	} else {
		http.FileServer(http.Dir("./")).ServeHTTP(w, r)
	}
}

func salaHandlerFunc(w *http.ResponseWriter, r *http.Request, usercookie *http.Cookie, nomeDaSala string) {
	sala, existe := salas[nomeDaSala]
	missingPlayer := 0
	alreadyUser := false
	cards := Cards{}
	userUUID, err := uuid.Parse(usercookie.Value)
	if err != nil {
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
		if i == len(sala.users) && alreadyUser {
			(*w).Write([]byte("Modo espectador ainda não implementado, peça para alguem compartilhar a tela."))
			return
		}
	} else {
		sala = Sala{
			name:   nomeDaSala,
			users:  [2]User{},
			images: [24]string{},
		}
		imagens, err := os.ReadDir("./Cara-a-cara/img/")
		if err != nil || len(imagens) < len(sala.images) {
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		}
		for i := len(imagens) - 1; i > 0; i-- {
			j, err := rand.Int(rand.Reader, big.NewInt(int64(i)))
			if err != nil {
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
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		}
		sala.users[missingPlayer].img = sala.images[int(i.Int64())]
		sala.users[missingPlayer].uuid = userUUID
		if err != nil {
			(*w).WriteHeader(http.StatusBadRequest)
			return
		}
		cards = Cards{
			Images:   sala.images,
			YourCard: sala.users[missingPlayer].img,
		}
	}
	imagesJson, err := json.Marshal(cards)
	if err != nil {
		(*w).WriteHeader(http.StatusInternalServerError)
		return
	}
	salas[sala.name] = sala
	(*w).Write(imagesJson)
	(*w).Header().Add("Content-Type", "application/json")
	(*w).WriteHeader(http.StatusOK)
}
