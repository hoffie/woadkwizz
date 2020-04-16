package main

import (
	"math/rand"
)

const ()

var (
	VOCALS                 = []rune("AAEEIIOOUUÄÖÜY")
	VOCALS_LEN             = len(VOCALS)
	CONSONANTS             = []rune("BCDFGHJKLMNPQRSTVWXZ")
	CONSONANTS_LEN         = len(CONSONANTS)
	WORD_NUM_VOCALS        = 4
	WORD_NUM_CONSONANTS    = 6
	WORD_NUM_SPACES        = 2
	WORD_NUM_LETTERS_TOTAL = WORD_NUM_VOCALS + WORD_NUM_CONSONANTS + WORD_NUM_SPACES
)

func pickRandomLetters() string {
	var runes []rune
	for x := 0; x < WORD_NUM_VOCALS; x++ {
		l := VOCALS[rand.Perm(VOCALS_LEN)[0]]
		if howOftenUsed(l, runes) >= 2 {
			// Don't let people draw the same letter more than twice,
			// so, repeat the run.
			x--
			continue
		}
		runes = append(runes, rune(l))
	}

	for x := 0; x < WORD_NUM_CONSONANTS; x++ {
		l := CONSONANTS[rand.Perm(CONSONANTS_LEN)[0]]
		if howOftenUsed(l, runes) >= 2 {
			// Don't let people draw the same letter more than twice,
			// so, repeat the run.
			x--
			continue
		}
		runes = append(runes, rune(l))
	}

	for x := 0; x < WORD_NUM_SPACES; x++ {
		runes = append(runes, 0x00a0) // non-breaking space
	}

	rand.Shuffle(len(runes), func(i, j int) {
		runes[i], runes[j] = runes[j], runes[i]
	})

	return string(runes)
}

func howOftenUsed(l rune, letters []rune) uint {
	var count uint
	for _, m := range letters {
		if l == m {
			count++
		}
	}
	return count
}
