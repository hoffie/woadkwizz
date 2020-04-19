package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"

	"github.com/gin-gonic/gin"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var PLAYER_NAME_RE *regexp.Regexp
var err4xx error

func init() {
	PLAYER_NAME_RE = regexp.MustCompile("^\\S(\\S| ){0,14}\\S$")
	err4xx = errors.New("client problem")
}

func serveIndex(c *gin.Context) {
	c.Request.URL.Path = "/"
	router.HandleContext(c)
}

func streamGameEvents(c *gin.Context) {
	game, err := getVerifiedGame(c)
	if err != nil {
		if err != err4xx {
			log.Printf("failed to find verified game: %s", err)
			c.AbortWithStatus(500)
		}
		return
	}

	clientChan := broker.NewClientChan(game.ID)
	defer broker.RemoveClientChan(game.ID, clientChan)
	c.Stream(func(w io.Writer) bool {
		c.SSEvent(<-clientChan, "")
		return true
	})
}

func startNewGame(c *gin.Context) {
	playerName, err := getVerifiedPlayerName(c)
	if err != nil {
		return
	}

	var game Game
	var player Player
	err = db.Transaction(func(tx *gorm.DB) error {
		game = Game{
			Token: generateToken(),
			Round: 1,
		}
		err := tx.Create(&game).Error
		if err != nil {
			return err
		}

		player = Player{
			Token: generateToken(),
			Game:  game,
			Name:  playerName,
			Round: 0,
		}
		err = tx.Create(&player).Error
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Printf("failed to create game or player: %s", err)
		c.AbortWithStatus(500)
		return
	}
	c.JSON(201, gin.H{
		"game_token":   game.Token,
		"player_token": player.Token,
	})
}

func joinGame(c *gin.Context) {
	playerName, err := getVerifiedPlayerName(c)
	if err != nil {
		return
	}

	game, err := getVerifiedGame(c)
	if err != nil {
		if err != err4xx {
			log.Printf("failed to find verified game: %s", err)
			c.AbortWithStatus(500)
		}
		return
	}

	phase, err := game.GetPhase()
	if err != nil {
		log.Printf("failed to getPhase: %s", err)
		c.AbortWithStatus(500)
		return
	}
	if game.Round != 1 || phase != GAME_PHASE_WAIT_FOR_READY {
		c.JSON(403, gin.H{"error": "cannot join after game start"})
		return
	}

	errNameAlreadyTaken := errors.New("name already taken")
	player := Player{
		Name:   playerName,
		GameID: game.ID,
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var num uint64
		err := tx.Model(&player).Where(player).Count(&num).Error
		if err != nil {
			return err
		}
		if num != 0 {
			return errNameAlreadyTaken
		}

		player.Token = generateToken()

		err = tx.Create(&player).Error
		if err != nil {
			return err
		}
		return nil
	})
	if err == errNameAlreadyTaken {
		c.JSON(400, gin.H{"error": "name already taken"})
		return
	}
	if err != nil {
		log.Printf("failed to create player: %s", err)
		c.AbortWithStatus(500)
		return
	}
	broker.Send(game.ID, "players")
	broker.Send(game.ID, "scoreboard")
	c.JSON(201, gin.H{
		"player_token": player.Token,
	})
}

func getVerifiedPlayerName(c *gin.Context) (string, error) {
	var p struct {
		Name string `json:"player_name" binding:"required"`
	}
	if err := c.BindJSON(&p); err != nil {
		c.JSON(400, gin.H{"error": "missing player_name"})
		return "", err
	}
	if !PLAYER_NAME_RE.MatchString(p.Name) {
		c.JSON(400, gin.H{"error": "invalid player_name"})
		return "", errors.New("invalid player_name")
	}
	return p.Name, nil
}

func getVerifiedGame(c *gin.Context) (Game, error) {
	var game Game
	err := db.First(&game, "token = ?", c.Param("game_token")).Error
	if err == gorm.ErrRecordNotFound {
		c.JSON(404, gin.H{"error": "invalid game_token"})
		return game, err4xx
	}
	return game, err
}

func getVerifiedPlayer(c *gin.Context) (Player, error) {
	var player Player
	game, err := getVerifiedGame(c)
	if err != nil {
		return player, err
	}
	err = db.First(&player, "token = ?", c.Param("player_token")).Error
	if err == gorm.ErrRecordNotFound {
		c.JSON(404, gin.H{"error": "invalid player_token"})
		return player, err4xx
	}
	if err != nil {
		return player, err
	}
	if game.ID != player.GameID {
		c.JSON(400, gin.H{"error": "player_token does not match associated game"})
		return player, err4xx
	}
	player.Game = game
	return player, nil
}

func getPlayerList(c *gin.Context) {
	game, err := getVerifiedGame(c)
	if err != nil {
		if err != err4xx {
			log.Printf("failed to find verified game: %s", err)
			c.AbortWithStatus(500)
		}
		return
	}

	var players []Player
	err = db.Model(&game).Related(&players).Error
	if err != nil {
		log.Printf("failed to query players: %s", err)
		c.AbortWithStatus(500)
		return
	}
	names := make([]string, len(players))
	for i, player := range players {
		names[i] = player.Name
	}
	c.JSON(200, gin.H{
		"players": names,
	})
}

func markPlayerReady(c *gin.Context) {
	player, err := getVerifiedPlayer(c)
	if err != nil {
		if err != err4xx {
			log.Printf("GetPhase failed: %s", err)
			c.AbortWithStatus(500)
		}
		return
	}

	phase, err := player.Game.GetPhase()
	if err != nil {
		log.Printf("GetPhase failed: %s", err)
		c.AbortWithStatus(500)
		return
	}
	if phase != GAME_PHASE_WAIT_FOR_READY {
		c.JSON(403, nil)
		return
	}

	player.Round = player.Game.Round
	err = db.Save(&player).Error
	if err != nil {
		log.Printf("markPlayerReady failed: %s", err)
		c.AbortWithStatus(500)
		return
	}

	ready, err := player.Game.PlayersReady()
	if err != nil {
		log.Printf("numNonReadyPlayers failed: %s", err)
		c.AbortWithStatus(500)
		return
	}
	if ready {
		// All players are ready, move to next round:
		err = startNewRound(&player.Game)
		if err != nil {
			log.Printf("markPlayerReady failed startNewRound: %s", err)
			c.AbortWithStatus(500)
			return
		}
	}
	broker.Send(player.Game.ID, "board")
	c.JSON(200, nil)
}

func startNewRound(game *Game) error {
	err := db.Transaction(func(tx *gorm.DB) error {
		var players []Player
		err := tx.Model(&game).Related(&players).Error
		if err != nil {
			return fmt.Errorf("startNewRound failed to get players: %s", err)
		}

		assignCard := func(player *Player) error {
			// Find unused card:
			var card Card
			q := tx.Table("cards")
			q = q.Select("cards.*, COUNT(words_usage.card_id) as usage_count")
			// Join the words table for calculating total card usage across all games:
			q = q.Joins("LEFT JOIN words AS words_usage ON words_usage.card_id = cards.id")
			// Join the words table a second time for finding out whether a card
			// was already used in this game:
			q = q.Joins("LEFT JOIN words ON words.card_id = cards.id AND words.game_id = ?", game.ID)
			// Required for the COUNT():
			q = q.Group("cards.id")
			// Only include cards which haven't been used in this game:
			q = q.Where("words.card_id IS NULL")
			// Pre-sort randomly:
			q = q.Order(gorm.Expr("random()"))
			// Perform the final sort based on total usage count, i.e.
			// prefer cards which have been used the least overall:
			q = q.Order("usage_count")
			// Pick one:
			q = q.Limit(1)
			err = q.First(&card).Error
			if err != nil {
				return fmt.Errorf("failed to assign card: %s", err)
			}
			// Save new word entry:
			word := Word{
				GameID:  game.ID,
				Round:   game.Round,
				CardID:  card.ID,
				Letters: pickRandomLetters(),
			}
			if player != nil {
				word.PlayerID = &player.ID
			} // else it's NULL

			err = tx.Save(&word).Error
			if err != nil {
				return fmt.Errorf("failed to save word entry: %s", err)
			}

			if player == nil {
				return nil
			}

			// Assign new word entry:
			player.Word = word
			err = tx.Save(&player).Error
			if err != nil {
				return fmt.Errorf("failed to assign new word entry: %s", err)
			}
			return nil
		}

		for _, player := range players {
			err := assignCard(&player)
			if err != nil {
				return err
			}
		}

		numAdditionalCards := len(players) + 1
		if len(players) <= 5 {
			numAdditionalCards = 6 - len(players)
		} else {
			numAdditionalCards = 1
		}
		for x := 0; x < numAdditionalCards; x++ {
			err := assignCard(nil)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

type jsonBoard struct {
	Players         []jsonPlayer        `json:"players"`
	Self            jsonSelf            `json:"self"`
	Round           int64               `json:"round"`
	Phase           string              `json:"phase"`
	Cards           []jsonCard          `json:"cards"`
	CurrentlyScored jsonCurrentlyScored `json:"currently_scored"`
	ScoreboardOrder []uint64            `json:"scoreboard_order"`
}

type jsonCard struct {
	ID       uint64  `json:"id"`
	IsSelf   bool    `json:"is_self"`
	Text     string  `json:"text"`
	PlayerID *uint64 `json:"player_id"`
	Score    *uint64 `json:"score"`
}

type jsonPlayer struct {
	ID                  uint64 `json:"id"`
	Name                string `json:"name"`
	IsReady             bool   `json:"is_ready"`
	IsSelf              bool   `json:"is_self"`
	Letters             string `json:"letters"`
	Word                string `json:"word"`
	ScoreTotal          uint64 `json:"score_total"`
	ScoreOwnWords       uint64 `json:"score_own_words"`
	ScoreCorrectGuesses uint64 `json:"score_correct_guesses"`
	AllWordsAssigned    bool   `json:"all_words_assigned"`
}

type jsonSelf struct {
	Card    jsonCard `json:"card"`
	Letters string   `json:"letters"`
	IsReady bool     `json:"is_ready"`
	Word    string   `json:"word"`
}

type jsonCurrentlyScored struct {
	PlayerID uint64      `json:"player_id"`
	Word     string      `json:"word"`
	Guesses  jsonGuesses `json:"guesses"`
}

type jsonGuesses map[uint64]uint64

type jsonScoreboard struct {
	Scoreboard []jsonScoreboardRow `json:"scoreboard"`
}

type jsonScoreboardRow struct {
	Name                string `json:"name"`
	ScoreTotal          uint64 `json:"score_total"`
	ScoreOwnWords       uint64 `json:"score_own_words"`
	ScoreCorrectGuesses uint64 `json:"score_correct_guesses"`
}

func getBoard(c *gin.Context) {
	player, err := getVerifiedPlayer(c)
	if err != nil {
		return
	}

	board, err := getBoardJson(player)
	if err != nil {
		log.Printf("getBoardJson failed: %v", err)
		c.AbortWithStatus(500)
		return
	}

	c.JSON(200, board)
}

func getBoardJson(player Player) (jsonBoard, error) {
	board := jsonBoard{}

	board.Round = player.Game.Round
	var err error
	board.Phase, err = player.Game.GetPhase()
	if err != nil {
		return board, err
	}

	var word Word
	var card Card
	err = db.Model(&player).Related(&word).Error
	if err == nil {
		err = db.Model(&word).Related(&card).Error
		if err != nil {
			return board, err
		}
		board.Self.Card = jsonCard{
			ID:       card.ID,
			Text:     card.Text,
			IsSelf:   true,
			PlayerID: &player.ID,
		}
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return board, err
	}

	scoreboardOrder, scoreByPlayer, err := getScoreByPlayers(player.Game.ID)
	if err != nil {
		return board, err
	}
	board.ScoreboardOrder = scoreboardOrder

	var players []Player
	err = db.Model(&player.Game).Related(&players).Error
	if err != nil {
		return board, err
	}

	wordsAssignedByPlayer, err := getWordsAssignedByPlayers(player.Game.ID, len(players))
	if err != nil {
		return board, err
	}

	for _, otherPlayer := range players {
		var word Word
		err = db.Model(&otherPlayer).Related(&word).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return board, err
		}
		isReady := otherPlayer.Round >= player.Game.Round
		board.Players = append(board.Players, jsonPlayer{
			ID:                  otherPlayer.ID,
			Name:                otherPlayer.Name,
			IsReady:             isReady,
			IsSelf:              otherPlayer.ID == player.ID,
			Letters:             word.Letters,
			Word:                word.Word,
			ScoreTotal:          scoreByPlayer[otherPlayer.ID].ScoreTotal,
			ScoreOwnWords:       scoreByPlayer[otherPlayer.ID].ScoreOwnWords,
			ScoreCorrectGuesses: scoreByPlayer[otherPlayer.ID].ScoreCorrectGuesses,
			AllWordsAssigned:    wordsAssignedByPlayer[otherPlayer.ID],
		})
		if otherPlayer.ID == player.ID {
			board.Self.IsReady = isReady
			board.Self.Letters = word.Letters
			board.Self.Word = word.Word
		}
	}

	var words []struct {
		CardID   uint64
		CardText string
		PlayerID *uint64
		IsScored bool
		Score    uint64
	}
	q := db.Table("words")
	q = q.Select("cards.id AS card_id, cards.text AS card_text, words.player_id AS player_id, words.is_scored AS is_scored, (COUNT(my_correct_guesses.id) + COUNT(correct_guesses_own_word.id)) AS score")
	q = q.Joins("LEFT JOIN cards ON words.card_id = cards.id LEFT JOIN guesses AS my_correct_guesses ON my_correct_guesses.word_id = words.id AND my_correct_guesses.card_id = words.card_id AND my_correct_guesses.player_id = ? AND words.is_scored = 1", player.ID)
	q = q.Joins("LEFT JOIN guesses AS correct_guesses_own_word ON correct_guesses_own_word.word_id = words.id AND words.player_id = ? AND correct_guesses_own_word.card_id = words.card_id AND words.is_scored = 1", player.ID)
	q = q.Where("words.game_id = ?", player.Game.ID)
	q = q.Where("words.round = ?", player.Round)
	q = q.Group("words.id")
	q = q.Order("words.id")
	q = q.Scan(&words)
	err = q.Error
	if err != nil {
		return board, err
	}
	board.Cards = make([]jsonCard, 0)
	for _, word := range words {
		c := jsonCard{
			ID:     word.CardID,
			Text:   word.CardText,
			IsSelf: word.CardID == board.Self.Card.ID,
		}
		if c.IsSelf || word.IsScored {
			c.PlayerID = word.PlayerID
		}
		if word.IsScored {
			// Create a copy which we can safely reference:
			score := word.Score
			c.Score = &score
		} else {
			c.Score = nil
		}
		board.Cards = append(board.Cards, c)
	}
	if board.Phase == GAME_PHASE_SCORE {
		word, err := getCurrentlyScoredWord(player.Game)
		if err != nil {
			return board, err
		}
		board.CurrentlyScored.Word = word.Word
		board.CurrentlyScored.PlayerID = *word.PlayerID
		board.CurrentlyScored.Guesses = make(jsonGuesses, 0)
		for _, guess := range word.Guesses {
			board.CurrentlyScored.Guesses[guess.PlayerID] = guess.CardID
		}
	}
	return board, nil
}

func getCurrentlyScoredWord(game Game) (Word, error) {
	var word Word
	var numPlayers int64
	p := Player{
		GameID: game.ID,
	}
	err := db.Model(&p).Where(p).Count(&numPlayers).Error
	if err != nil {
		return word, err
	}
	q := db.Model(&word)
	q = q.Preload("Guesses")
	q = q.Where("game_id = ?", game.ID)
	q = q.Where("player_id IS NOT NULL")
	q = q.Where("is_scored = 0")
	q = q.Order("card_id")
	err = q.First(&word).Error
	if err != nil {
		return word, err
	}
	return word, nil
}

func submitWord(c *gin.Context) {
	player, err := getVerifiedPlayer(c)
	if err != nil {
		if err != err4xx {
			log.Printf("getVerifiedPlayer failed: %s", err)
			c.AbortWithStatus(500)
		}
		return
	}

	phase, err := player.Game.GetPhase()
	if err != nil {
		log.Printf("GetPhase failed: %s", err)
		c.AbortWithStatus(500)
		return
	}
	if phase != GAME_PHASE_SUBMIT_WORD {
		c.JSON(403, nil)
		return
	}
	var word Word
	err = db.Model(player).Related(&word).Error
	if err != nil {
		return
	}

	var w struct {
		Word string `json:"word" binding:"required"`
	}
	if err := c.BindJSON(&w); err != nil {
		c.JSON(400, gin.H{"error": "no word submitted"})
		return
	}

	if len(w.Word) > WORD_NUM_LETTERS_TOTAL {
		c.JSON(400, gin.H{"error": "too many letters"})
		return
	}

	availableLetters := word.Letters
	for _, wl := range w.Word {
		found := false
		for i, al := range availableLetters {
			if wl == al {
				// Remove from availableLetters to prevent duplicate usage:
				availableLetters = availableLetters[:i] + availableLetters[i+1:]
				found = true
				break
			}
		}
		if !found {
			c.JSON(400, gin.H{"error": "invalid letter"})
			return
		}
	}

	word.Word = w.Word
	err = db.Save(&word).Error
	if err != nil {
		log.Printf("failed to save word: %s", err)
		c.AbortWithStatus(500)
		return
	}

	broker.Send(player.Game.ID, "board")
	c.JSON(200, nil)
}

func submitGuesses(c *gin.Context) {
	player, err := getVerifiedPlayer(c)
	if err != nil {
		if err != err4xx {
			log.Printf("getVerifiedPlayer failed: %s", err)
			c.AbortWithStatus(500)
		}
		return
	}

	phase, err := player.Game.GetPhase()
	if err != nil {
		log.Printf("GetPhase failed: %s", err)
		c.AbortWithStatus(500)
		return
	}
	if phase != GAME_PHASE_ASSIGN_WORDS {
		c.JSON(403, nil)
		return
	}
	var guesses struct {
		Guesses jsonGuesses `json:"guesses" binding:"required"`
	}
	if err := c.BindJSON(&guesses); err != nil {
		c.JSON(400, gin.H{"error": "missing guesses"})
		return
	}

	var numOtherPlayers int
	q := db.Table("players")
	q = q.Where("game_id = ?", player.GameID)
	q = q.Where("id <> ?", player.ID)
	q = q.Count(&numOtherPlayers)
	err = q.Error
	if err != nil {
		log.Printf("failed to get number of other players: %s", err)
		c.AbortWithStatus(500)
		return
	}
	if len(guesses.Guesses) != numOtherPlayers {
		c.JSON(400, gin.H{"error": "bad number of guesses"})
		return
	}
	for playerID, _ := range guesses.Guesses {
		if playerID == player.ID {
			c.JSON(403, gin.H{"error": "attempting to guess own word"})
			return
		}
	}

	errCardUsed := errors.New("card already used")
	errInvalidCard := errors.New("invalid card")
	err = db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where(Guess{
			GameID:   player.Game.ID,
			Round:    player.Game.Round,
			PlayerID: player.ID,
		}).Delete(Guess{}).Error
		if err != nil {
			return err
		}
		usedCards := make(map[uint64]bool, 0)
		for playerID, cardID := range guesses.Guesses {
			if _, exists := usedCards[cardID]; exists {
				return errCardUsed
			}
			var word Word
			err = tx.Where(Word{
				GameID:   player.Game.ID,
				Round:    player.Game.Round,
				PlayerID: &playerID,
			}).First(&word).Error
			if err != nil {
				return err
			}

			var validCard uint64
			err = tx.Table("words").Where(&Word{
				GameID: player.Game.ID,
				Round:  player.Game.Round,
				CardID: cardID,
			}).Count(&validCard).Error
			if err != nil {
				return err
			}
			if validCard != 1 {
				return errInvalidCard
			}

			err = tx.Save(&Guess{
				GameID:   player.Game.ID,
				Round:    player.Game.Round,
				PlayerID: player.ID,
				WordID:   word.ID,
				CardID:   cardID,
			}).Error
			if err != nil {
				return err
			}
			usedCards[cardID] = true
		}
		return nil
	})
	if err == gorm.ErrRecordNotFound {
		c.JSON(400, gin.H{"error": "player not resolvable to word"})
		return
	}
	if err == errInvalidCard {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err == errCardUsed {
		c.JSON(400, gin.H{"error": "duplicate card use"})
		return
	}
	if err != nil {
		log.Printf("submitGuesses failed: %v", err)
		c.AbortWithStatus(500)
		return
	}

	broker.Send(player.Game.ID, "board")
	c.JSON(200, nil)
}

func getGuesses(c *gin.Context) {
	player, err := getVerifiedPlayer(c)
	if err != nil {
		if err != err4xx {
			log.Printf("getVerifiedPlayer failed: %s", err)
			c.AbortWithStatus(500)
		}
		return
	}

	phase, err := player.Game.GetPhase()
	if err != nil {
		log.Printf("GetPhase failed: %s", err)
		c.AbortWithStatus(500)
		return
	}
	if phase != GAME_PHASE_ASSIGN_WORDS {
		c.JSON(403, nil)
		return
	}

	var guesses []Guess
	q := db.Preload("Word")
	q = q.Where("game_id = ?", player.Game.ID)
	q = q.Where("round = ?", player.Game.Round)
	q = q.Where("player_id = ?", player.ID)
	err = q.Find(&guesses).Error
	if err != nil {
		c.AbortWithStatus(500)
		return
	}
	g := make(jsonGuesses, 0)
	for _, guess := range guesses {
		if guess.Word.PlayerID == nil {
			continue
		}
		g[*guess.Word.PlayerID] = guess.CardID
	}

	c.JSON(200, gin.H{"guesses": g})
}

func markScored(c *gin.Context) {
	player, err := getVerifiedPlayer(c)
	if err != nil {
		if err != err4xx {
			log.Printf("failed to get verified player: %s", err)
			c.AbortWithStatus(500)
		}
		return
	}

	phase, err := player.Game.GetPhase()
	if err != nil {
		log.Printf("GetPhase failed: %s", err)
		c.AbortWithStatus(500)
		return
	}
	if phase != GAME_PHASE_SCORE {
		c.JSON(403, gin.H{"error": "wrong game phase"})
		return
	}

	word, err := getCurrentlyScoredWord(player.Game)
	if err == gorm.ErrRecordNotFound {
		c.JSON(403, gin.H{"error": "no scoring in progress"})
		return
	}
	if err != nil {
		log.Printf("getCurrentlyScoredWord failed: %s", err)
		c.AbortWithStatus(500)
		return
	}

	if word.PlayerID != nil && *word.PlayerID != player.ID {
		c.JSON(403, gin.H{"error": "not your turn"})
		return
	}

	word.IsScored = true

	err = db.Save(&word).Error
	if err != nil {
		log.Printf("saving word failed: %s", err)
		c.AbortWithStatus(500)
		return
	}

	word, err = getCurrentlyScoredWord(player.Game)
	if err == gorm.ErrRecordNotFound {
		game := player.Game
		game.Round++
		err := db.Save(&game).Error
		if err != nil {
			log.Printf("failed to save new round number: %s", err)
			c.AbortWithStatus(500)
			return
		}
	}

	broker.Send(player.Game.ID, "board")
	broker.Send(player.Game.ID, "scoreboard")
	c.JSON(200, nil)
}

type scoreByPlayer struct {
	PlayerID            uint64
	Name                string
	ScoreTotal          uint64
	ScoreOwnWords       uint64
	ScoreCorrectGuesses uint64
}

func getScoreByPlayers(gameID uint64) ([]uint64, map[uint64]scoreByPlayer, error) {
	resultsByPlayer := make(map[uint64]scoreByPlayer, 0)
	var resultOrder []uint64
	var results []scoreByPlayer
	q := db.Table("players")
	q = q.Select("players.id AS player_id, players.name, COUNT(DISTINCT correct_guesses_words.id) AS score_correct_guesses, COUNT(DISTINCT own_words_guesses.id) AS score_own_words, (COUNT(DISTINCT correct_guesses_words.id) + COUNT(DISTINCT own_words_guesses.id)) AS score_total")
	q = q.Joins("LEFT JOIN guesses AS correct_guesses ON correct_guesses.player_id = players.id")
	q = q.Joins("LEFT JOIN words AS correct_guesses_words ON correct_guesses.word_id = correct_guesses_words.id AND correct_guesses_words.is_scored = 1 AND correct_guesses.card_id = correct_guesses_words.card_id")
	q = q.Joins("LEFT JOIN words AS own_words ON own_words.player_id = players.id")
	q = q.Joins("LEFT JOIN guesses AS own_words_guesses ON own_words.id = own_words_guesses.word_id AND own_words.card_id = own_words_guesses.card_id AND own_words.is_scored = 1")
	q = q.Where("players.game_id = ?", gameID)
	q = q.Group("players.id")
	q = q.Order("score_total DESC, score_own_words DESC, score_correct_guesses DESC, players.id")
	q = q.Scan(&results)
	err := q.Error
	if err != nil {
		return resultOrder, resultsByPlayer, err
	}
	for _, result := range results {
		resultOrder = append(resultOrder, result.PlayerID)
		resultsByPlayer[result.PlayerID] = result
	}
	return resultOrder, resultsByPlayer, nil
}

func getWordsAssignedByPlayers(gameID uint64, numPlayers int) (map[uint64]bool, error) {
	resultsByPlayer := make(map[uint64]bool, 0)
	var results []struct {
		ID               uint64
		AllWordsAssigned bool
	}
	q := db.Table("players")
	q = q.Select("players.*, (COUNT(guesses.id) == ?) AS all_words_assigned", numPlayers-1)
	q = q.Joins("LEFT JOIN guesses ON guesses.game_id = players.game_id AND guesses.round = players.round AND guesses.player_id = players.id")
	q = q.Where("players.game_id = ?", gameID)
	q = q.Group("players.id, guesses.player_id")
	q = q.Scan(&results)
	err := q.Error
	if err != nil {
		return resultsByPlayer, err
	}
	for _, result := range results {
		resultsByPlayer[result.ID] = result.AllWordsAssigned
	}
	return resultsByPlayer, nil
}
