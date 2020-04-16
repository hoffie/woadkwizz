package main

import (
	"time"
)

const (
	// GAME_PHASE_* is used to denote different situations.
	// These values are calculated and not saved in the db.
	// They are returned by Game.GetPhase()
	GAME_PHASE_WAIT_FOR_READY = "wait-for-ready"
	GAME_PHASE_SUBMIT_WORD    = "submit-word"
	GAME_PHASE_ASSIGN_WORDS   = "assign-words"
	GAME_PHASE_SCORE          = "score"
)

type Card struct {
	ID   uint64
	Text string `gorm:"unique; not null"`
}

type Game struct {
	ID        uint64
	Token     string `gorm:"unique; not null"`
	Round     int64
	CreatedAt time.Time
}

func (g Game) GetPhase() (string, error) {
	playersReady, err := g.PlayersReady()
	if err != nil {
		return "", err
	}
	if !playersReady {
		return GAME_PHASE_WAIT_FOR_READY, nil
	}
	var numUnsubmittedWords uint64
	q := db.Table("players")
	q = q.Joins("LEFT JOIN words ON words.player_id = players.id AND words.game_id = players.game_id AND words.round = players.round")
	q = q.Where("players.game_id = ?", g.ID)
	q = q.Where("words.word = ''")
	q = q.Count(&numUnsubmittedWords)
	err = q.Error
	if err != nil {
		return "", err
	}
	if numUnsubmittedWords != 0 {
		return GAME_PHASE_SUBMIT_WORD, nil
	}

	var numPlayersWithUnassignedWords uint64
	q = db.Table("players")
	q = q.Select("players.*, COUNT(guesses.id) as guess_count")
	q = q.Joins("LEFT JOIN guesses ON guesses.game_id = players.game_id AND guesses.round = players.round AND guesses.player_id = players.id")
	q = q.Where("players.game_id = ?", g.ID)
	q = q.Group("players.id, guesses.player_id")
	q = q.Having("guess_count <> (SELECT COUNT(all_players.id)-1 FROM players AS all_players WHERE all_players.game_id = players.game_id)")
	q = q.Count(&numPlayersWithUnassignedWords)
	err = q.Error
	if err != nil {
		return "", err
	}

	if numPlayersWithUnassignedWords != 0 {
		return GAME_PHASE_ASSIGN_WORDS, nil
	}

	return GAME_PHASE_SCORE, nil
}

func (g Game) PlayersReady() (bool, error) {
	var numPlayers uint64
	err := db.Table("players").Where("game_id = ?", g.ID).Count(&numPlayers).Error
	if err != nil {
		return false, err
	}
	if numPlayers < 3 {
		// Game doesn't work with less than 3 players.
		return false, nil
	}

	var numNonreadyPlayers uint64
	err = db.Table("players").Where("game_id = ?", g.ID).Where("round < ?", g.Round).Count(&numNonreadyPlayers).Error
	if err != nil {
		return false, err
	}
	return numNonreadyPlayers == 0, err
}

type Player struct {
	ID     uint64
	GameID uint64 `gorm:"unique_index:idx_gameid_name; not null"`
	Game   Game
	Token  string `gorm:"unique; not null"`
	Name   string `gorm:"unique_index:idx_gameid_name; not null"`
	Round  int64
	Word   Word `gorm:"association_foreignkey:PlayerID,GameID,Round"`
}

type Word struct {
	ID       uint64
	GameID   uint64 `gorm:"unique_index:idx_gameid_cardid; not null"`
	Game     Game
	Round    int64
	CardID   uint64 `gorm:"unique_index:idx_gameid_cardid; not null"`
	Card     Card
	PlayerID *uint64 // Will be NULL in case of the additional card.
	Word     string
	Letters  string
	IsScored bool
	Guesses  []Guess
}

type Guess struct {
	ID       uint64
	GameID   uint64 `gorm:"unique_index:idx_gameid_round_playerid_wordid_cardid; not null"`
	Round    int64  `gorm:"unique_index:idx_gameid_round_playerid_wordid_cardid; not null"`
	PlayerID uint64 `gorm:"unique_index:idx_gameid_round_playerid_wordid_cardid; not null"`
	Player   Player
	WordID   uint64 `gorm:"unique_index:idx_gameid_round_playerid_wordid_cardid; not null"`
	Word     Word
	CardID   uint64 `gorm:"unique_index:idx_gameid_round_playerid_wordid_cardid; not null"`
	Card     Card
}
