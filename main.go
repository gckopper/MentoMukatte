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
	port := flag.Int("p", 3669, "Port in which the updater will listen")
	flag.Parse()
	println("Hello, World!")
	http.Handle("/", http.HandlerFunc(generalHandlerFunc))      //http.FileServer(http.Dir("./"))
	http.Handle("/status", http.HandlerFunc(statusHandlerFunc)) //http.FileServer(http.Dir("./"))
	err := http.ListenAndServe(fmt.Sprint("localhost:", *port), nil)
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
			Secure:  false,
			Name:    "SessionCookie",
			Value:   uuid.New().String(),
			MaxAge:  31536000,
			Expires: time.Now().Add(time.Hour * 24 * 365), // Give it a year of life
			//SameSite: http.SameSiteStrictMode,              // Set SameSite to strict as a way of mitigating attacks
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
		http.FileServer(http.Dir(".\\Cara-a-cara\\")).ServeHTTP(w, r)
	}
}

func salaHandlerFunc(w *http.ResponseWriter, r *http.Request, usercookie *http.Cookie, nomeDaSala string) {
	sala, existe := salas[nomeDaSala]
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
		sala = Sala{
			name:   nomeDaSala,
			users:  [2]User{},
			images: [24]string{},
		}
		imagens, err := os.ReadDir("./Cara-a-cara/img/")
		if err != nil || len(imagens) < len(sala.images) {
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
			log.Println(err)
			(*w).WriteHeader(http.StatusInternalServerError)
			return
		}
		sala.users[missingPlayer].img = sala.images[int(i.Int64())]
		sala.users[missingPlayer].uuid = userUUID
		if err != nil {
			log.Println(err)
			(*w).WriteHeader(http.StatusBadRequest)
			return
		}
		cards = Cards{
			Images:   sala.images,
			YourCard: sala.users[missingPlayer].img,
		}
	}
	t, err := template.ParseFiles("Cara-a-cara/index.html")
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
	salas[sala.name] = sala

}

func deleteHandlerFunc(w *http.ResponseWriter, r *http.Request) {
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err == http.ErrNoCookie {
		(*w).WriteHeader(http.StatusBadRequest)
	}
	if err != http.ErrNoCookie && err != nil {
		(*w).WriteHeader(http.StatusInternalServerError)
	}
	userUUID, err := uuid.Parse(userCookie.Value)
	if err != nil {
		log.Println(err)
		(*w).WriteHeader(http.StatusBadRequest)
		return
	}
	for _, sala := range salas {
		for _, v := range sala.users {
			if v.uuid == userUUID {
				delete(salas, sala.name)
				break
			}
		}
	}
}

func statusHandlerFunc(w http.ResponseWriter, r *http.Request) {
	userCookie, err := r.Cookie("SessionCookie") // Try to grab the cookie named SessionCookie
	if err == http.ErrNoCookie {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No cookie"))
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
	nomeDaSala := r.URL.Query().Get("sala")
	if nomeDaSala == "" {
		log.Println("não mandou o nome da sala")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sala, existe := salas[nomeDaSala]
	if !existe {
		log.Println("A sala não existe")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for i, v := range sala.users {
		if v.uuid == userUUID {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			sala.users[i].dead, err = strconv.Atoi(string(body))
			if err != nil {
				log.Println(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if i == 0 {
				w.Write([]byte(fmt.Sprint(sala.users[1].dead)))
			} else {
				w.Write([]byte(fmt.Sprint(sala.users[0].dead)))
			}
			salas[sala.name] = sala
			break
		}
	}
}
