package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type Card struct {
	Code  string `json:"code"`
	Image string `json:"image"`
	Rank  string `json:"rank"`
	Suit  string `json:"suit"`
}

type Deck struct {
	ID        string `json:"deck_id"`
	Remaining int    `json:"remaining"`
	Cards     []Card `json:"cards,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Request struct {
	SQL             string
	ResponseChannel chan Response
}

type Response struct {
	Result interface{}
	Error  error
}

var requestChannel chan Request // Afin d'avoir accès dans tout mon code

func main() {

	db := initDB()
	requestChannel = make(chan Request)
	go databaseManager(requestChannel, db)

	http.HandleFunc("/deck/", deckHandler)
	http.HandleFunc("/static/", afficherImage)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initDB() *sql.DB {
	database, err := sql.Open("sqlite3", "./tp1.db")
	if err != nil {
		log.Fatal(err)
	}
	statement, err := database.Prepare("CREATE TABLE IF NOT EXISTS decks (id TEXT PRIMARY KEY, cards TEXT)")
	if err != nil {
		log.Fatal(err)
	}
	statement.Exec()
	return database
}

func deckHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("DeckHandler called. Received a request for", r.URL.Path)
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) < 3 {
		http.NotFound(w, r)
		return
	}

	if segments[1] == "deck" {
		action := segments[2]

		switch action {
		case "new":
			createDeck(w, r)
		case "add":
			addCards(w, r)
		case "draw":
			drawCards(w, r)
		case "shuffle":
			shuffleDeck(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func databaseManager(requestChannel chan Request, db *sql.DB) { // Boucle infinie, je ne voyais pas d'autre façon de le faire et je voulais simplifier mon utilisation du channel
	log.Println("Database manager started.")
	for req := range requestChannel {
		go func(req Request) {
			log.Println("About to execute SQL:", req.SQL)
			var result interface{}
			var err error

			if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(req.SQL)), "SELECT") {
				result, err = db.Query(req.SQL)
			} else {
				result, err = db.Exec(req.SQL)
			}

			if err != nil {
				log.Println("SQL Query Failed:", err)
			} else {
				log.Println("SQL Query Succeeded.")
			}

			req.ResponseChannel <- Response{Result: result, Error: err}
			log.Println("Sent response back.")
		}(req)
	}
	log.Println("Database manager stopped.")
}

func createDeck(w http.ResponseWriter, r *http.Request) {
	countParam := r.URL.Query().Get("count")
	if countParam == "" {
		countParam = "1"
	}
	count, err := strconv.Atoi(countParam)
	if err != nil {
		http.Error(w, "Le paramètre 'count' doit être un entier", http.StatusBadRequest)
		return
	}

	var newDecks []Deck
	for i := 0; i < count; i++ {
		newDeckID := uuid.NewString()
		cards := generateStandardDeck()
		cardsJSON, err := json.Marshal(cards)
		if err != nil {
			http.Error(w, "Erreur lors de la conversion des cartes en JSON", http.StatusInternalServerError)
			return
		}

		responseChannel := make(chan Response)
		requestChannel <- Request{
			SQL:             fmt.Sprintf("INSERT INTO decks (id, cards) VALUES ('%s', '%s')", newDeckID, string(cardsJSON)),
			ResponseChannel: responseChannel,
		}

		response := <-responseChannel
		if response.Error != nil {
			http.Error(w, "Erreur lors de l'insertion du nouveau paquet dans la base de données", http.StatusInternalServerError)
			return
		}

		newDecks = append(newDecks, Deck{
			ID:        newDeckID,
			Remaining: len(cards),
		})
	}
	json.NewEncoder(w).Encode(newDecks)
}

func generateStandardDeck() []Card {
	var deck []Card
	suits := []string{"h", "d", "c", "s"}
	for _, suit := range suits {
		for i := 1; i <= 13; i++ {
			card := Card{
				Code:  fmt.Sprintf("%d%s", i, suit),
				Image: fmt.Sprintf("/static/%d%s.png", i, suit),
				Rank:  strconv.Itoa(i),
				Suit:  suit,
			}
			deck = append(deck, card)
		}
	}
	return deck
}

func addCards(w http.ResponseWriter, r *http.Request) {

	segments := strings.Split(r.URL.Path, "/")
	if len(segments) < 4 {
		http.Error(w, "ID de deck manquant dans l'URL", http.StatusBadRequest)
		return
	}
	deckID := segments[3]

	// Extraire les cartes à ajouter à partir de la requête
	cardsParam := r.URL.Query().Get("cards")
	if cardsParam == "" {
		http.Error(w, "Paramètre 'cards' manquant", http.StatusBadRequest)
		return
	}
	cardsToAdd := strings.Split(cardsParam, ",")

	// Convertir les cartes en JSON pour la mise à jour dans la base de données
	cardsJSON, err := json.Marshal(cardsToAdd)
	if err != nil {
		http.Error(w, "Erreur lors de la conversion des cartes en JSON", http.StatusInternalServerError)
		return
	}

	// Créer un channel de réponse pour cette requête
	responseChannel := make(chan Response)

	// Mettre à jour le deck dans la base de données
	requestChannel <- Request{
		SQL:             fmt.Sprintf("UPDATE decks SET cards = json_insert(cards, '$', json('%s')) WHERE id = '%s'", string(cardsJSON), deckID),
		ResponseChannel: responseChannel,
	}

	response := <-responseChannel
	if response.Error != nil {
		http.Error(w, "Erreur lors de l'ajout de cartes au deck dans la base de données", http.StatusInternalServerError)
		return
	}

	// Envoyer une réponse de succès
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Cartes ajoutées avec succès"})
}

func drawCards(w http.ResponseWriter, r *http.Request) {
	// Extraire l'ID du deck à partir de l'URL
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) < 4 {
		http.Error(w, "ID de deck manquant dans l'URL", http.StatusBadRequest)
		return
	}
	deckID := segments[3]

	nbrCarte := r.URL.Query().Get("nbrCarte")
	if nbrCarte == "" {
		nbrCarte = "1"
	}
	nbrCarteInt, err := strconv.Atoi(nbrCarte)
	if err != nil {
		http.Error(w, "Le nombre de cartes doit être un entier", http.StatusBadRequest)
		return
	}

	// Créer un channel de réponse pour cette requête
	responseChannel := make(chan Response)

	// Récupérer les cartes du deck dans la base de données
	requestChannel <- Request{
		SQL:             fmt.Sprintf("SELECT cards FROM decks WHERE id = '%s'", deckID),
		ResponseChannel: responseChannel,
	}

	// Attendre la réponse
	response := <-responseChannel
	if response.Error != nil {
		http.Error(w, "Erreur lors de la récupération du deck dans la base de données", http.StatusInternalServerError)
		return
	}

	rows, ok := response.Result.(*sql.Rows)
	if !ok {
		http.Error(w, "Erreur lors de la conversion du résultat en sql.Rows", http.StatusInternalServerError)
		return
	}

	var cardsJSON string
	for rows.Next() {
		err := rows.Scan(&cardsJSON)
		if err != nil {
			http.Error(w, "Erreur lors de la lecture des cartes du deck", http.StatusInternalServerError)
			return
		}
	}

	var cards []Card
	err = json.Unmarshal([]byte(cardsJSON), &cards)
	if err != nil {
		http.Error(w, "Erreur lors de la conversion du JSON en slice de cartes : "+err.Error(), http.StatusInternalServerError)
		return
	}

	if nbrCarteInt > len(cards) {
		http.Error(w, "Pas assez de cartes dans le deck", http.StatusBadRequest)
		return
	}

	// Tirer les cartes
	drawnCards := cards[:nbrCarteInt]
	remainingCards := cards[nbrCarteInt:]

	// Mettre à jour le deck dans la base de données
	remainingCardsJSON, err := json.Marshal(remainingCards)
	if err != nil {
		http.Error(w, "Erreur lors de la conversion des cartes restantes en JSON", http.StatusInternalServerError)
		return
	}

	requestChannel <- Request{
		SQL:             fmt.Sprintf("UPDATE decks SET cards = '%s' WHERE id = '%s'", string(remainingCardsJSON), deckID),
		ResponseChannel: responseChannel,
	}

	response = <-responseChannel
	if response.Error != nil {
		http.Error(w, "Erreur lors de la mise à jour du deck dans la base de données", http.StatusInternalServerError)
		return
	}

	// Renvoyer les cartes tirées
	json.NewEncoder(w).Encode(Deck{
		ID:        deckID,
		Remaining: len(remainingCards),
		Cards:     drawnCards,
	})
}

func shuffleDeck(w http.ResponseWriter, r *http.Request) {
	// Extraire l'ID du deck à partir de l'URL
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) < 4 {
		http.Error(w, "ID de deck manquant dans l'URL", http.StatusBadRequest)

		return
	}
	deckID := segments[3]

	// Créer un channel de réponse pour cette requête
	responseChannel := make(chan Response)

	// Récupérer les cartes du deck dans la base de données
	requestChannel <- Request{
		SQL:             fmt.Sprintf("SELECT cards FROM decks WHERE id = '%s'", deckID),
		ResponseChannel: responseChannel,
	}

	response := <-responseChannel
	if response.Error != nil {
		http.Error(w, "Erreur lors de la récupération du deck dans la base de données", http.StatusInternalServerError)
		return
	}

	rows, ok := response.Result.(*sql.Rows)
	if !ok {
		http.Error(w, "Erreur lors de la conversion du résultat en sql.Rows", http.StatusInternalServerError)
		return
	}

	var cardsJSON string
	for rows.Next() {
		err := rows.Scan(&cardsJSON)
		if err != nil {
			http.Error(w, "Erreur lors de la lecture des cartes du deck", http.StatusInternalServerError)
			return
		}
	}

	var cards []Card
	err := json.Unmarshal([]byte(cardsJSON), &cards)
	if err != nil {
		http.Error(w, "Erreur lors de la conversion du JSON en slice de cartes", http.StatusInternalServerError)
		return
	}

	// Mélanger les cartes
	time.Now().UnixNano()
	rand.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })

	shuffledCardsJSON, err := json.Marshal(cards)
	if err != nil {
		http.Error(w, "Erreur lors de la conversion des cartes mélangées en JSON", http.StatusInternalServerError)
		return
	}

	requestChannel <- Request{
		SQL:             fmt.Sprintf("UPDATE decks SET cards = '%s' WHERE id = '%s'", string(shuffledCardsJSON), deckID),
		ResponseChannel: responseChannel,
	}

	response = <-responseChannel
	if response.Error != nil {
		http.Error(w, "Erreur lors de la mise à jour du deck dans la base de données", http.StatusInternalServerError)
		return
	}

	// Renvoyer le nombre de cartes restantes
	json.NewEncoder(w).Encode(Deck{
		ID:        deckID,
		Remaining: len(cards),
	})
}

func afficherImage(w http.ResponseWriter, r *http.Request) {
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) < 3 {
		http.NotFound(w, r)
		return
	}
	cardCode := segments[2]
	imagePath := fmt.Sprintf("./static/%s.png", cardCode)

	http.ServeFile(w, r, imagePath)
}
