package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/gin-contrib/pprof"
)

var db *gorm.DB
var router *gin.Engine
var broker *Broker

var cardsPath string
var dbPath string
var assetsPath string
var listen string
var debug bool

func init() {
	flag.StringVar(&cardsPath, "cardsPath", "cards.b64", "path to the file containing card texts")
	flag.StringVar(&dbPath, "dbPath", "game.sqlite", "path to the file containing sqlite database")
	flag.StringVar(&assetsPath, "assetsPath", "./ui", "path to the directory containing static files")
	flag.StringVar(&listen, "listen", "127.0.0.1:3000", "host:port to listen on")
	flag.BoolVar(&debug, "debug", false, "whether to enable debugging features")
}

func main() {
	rand.Seed(time.Now().UnixNano())

	flag.Parse()

	broker = NewBroker()

	var err error
	db, err = gorm.Open("sqlite3", dbPath)
	if err != nil {
		panic("failed to connect database")
	}
	defer db.Close()
	db.AutoMigrate(&Game{})
	db.AutoMigrate(&Player{})
	db.AutoMigrate(&Card{})
	db.AutoMigrate(&Word{})
	db.AutoMigrate(&Guess{})

	importBaseCards()

	if !debug {
		gin.SetMode(gin.ReleaseMode)
	}
	router = gin.Default()
	if debug {
		pprof.Register(router)
	}

	router.StaticFile("/", filepath.Join(assetsPath, "index.html"))
	router.GET("/games/:game_token", serveIndex)
	router.GET("/games/:game_token/players/:player_token", serveIndex)
	router.Static("/ui", assetsPath)
	router.POST("/api/games", startNewGame)
	router.GET("/api/games/:game_token/events", streamGameEvents)
	router.POST("/api/games/:game_token/players", joinGame)
	router.GET("/api/games/:game_token/players", getPlayerList)
	router.GET("/api/games/:game_token/scoreboard", getScoreboard)
	router.PUT("/api/games/:game_token/players/:player_token/ready", markPlayerReady)
	router.GET("/api/games/:game_token/players/:player_token", getBoard)
	router.PUT("/api/games/:game_token/players/:player_token/word", submitWord)
	router.GET("/api/games/:game_token/players/:player_token/guesses", getGuesses)
	router.PUT("/api/games/:game_token/players/:player_token/guesses", submitGuesses)
	router.PUT("/api/games/:game_token/players/:player_token/scored", markScored)
	router.Run(listen)
}

func importBaseCards() {
	file, err := os.Open(cardsPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Card texts are base64-encoded in order to avoid indexing/blocking of
	// NSFW words.
	dec := base64.NewDecoder(base64.StdEncoding, file)

	scanner := bufio.NewScanner(dec)
	err = db.Transaction(func(tx *gorm.DB) error {
		// usage of a Transaction is important for performance as
		// multiple INSERTs will take a lot of time otherwise.
		for scanner.Scan() {
			var c Card
			card := Card{Text: scanner.Text()}
			err = tx.Where(card).FirstOrCreate(&c).Error
			if err != nil {
				return err
			}
		}

		if err = scanner.Err(); err != nil {
			log.Fatalf("card scanning failed: %s", err)
		}

		return nil
	})

	if err != nil {
		log.Fatalf("base card import failed: %s", err)
	}
}
