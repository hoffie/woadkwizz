#!/usr/bin/env python3
import sys
import time
import string
import unittest
import requests

SERVER = 'http://127.0.0.1:3000'


class TestUI(unittest.TestCase):
    def test_index(self):
        r = requests.get('%s/' % SERVER)
        self.assertTrue('<html>' in r.text)

    def test_player_view(self):
        r = requests.get('%s/games/game_token/players/player_token' % SERVER)
        self.assertTrue('<html>' in r.text)

    def test_join_view(self):
        r = requests.get('%s/games/game_token' % SERVER)
        self.assertTrue('<html>' in r.text)


class TestAPI(unittest.TestCase):
    def setUp(self):
        self.player_token = {}

    def api(self, path):
        return '%s/api%s' % (SERVER, path)

    def test_new_game(self):
        p = '/games'
        r = requests.post(self.api(p), json={
            'player_name': 'Player 1',
        })
        self.assertEqual(r.status_code, 201)
        j = r.json()
        self.game_token = j.get('game_token')
        self.player_token[0] = j.get('player_token')
        self.assertRegex(self.game_token, '[a-z0-9]{12}')
        self.assertRegex(self.player_token[0], '[a-z0-9]{12}')

    def test_player_list(self):
        self.test_new_game()
        p = '/games/%s/players' % self.game_token
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        players = r.json()['players']
        self.assertEqual(players, ['Player 1'])

    def test_game_start_wait_for_3_players(self):
        self.test_new_game()
        p = '/games/%s/players' % self.game_token
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        players = r.json()['players']
        self.assertEqual(players, ['Player 1'])

        r = requests.post(self.api(p), json={
            'player_name': 'Player 2',
        })
        self.assertEqual(r.status_code, 201)
        self.player_token[1] = r.json()['player_token']

        for player_token in self.player_token.values():
            p = '/games/%s/players/%s/ready' % (self.game_token, player_token)
            r = requests.put(self.api(p))
            self.assertEqual(r.status_code, 200)

        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['self']['is_ready'], True)
        self.assertEqual(j['players'][0]['is_ready'], True)
        self.assertEqual(j['players'][1]['is_ready'], True)
        self.assertEqual(len(j['players']), 2)
        self.assertEqual(j['phase'], 'wait-for-ready') # must not be submit-words yet!

        p = '/games/%s/players' % self.game_token
        r = requests.post(self.api(p), json={
            'player_name': 'Player 3',
        })
        self.assertEqual(r.status_code, 201)
        self.player_token[2] = r.json()['player_token']

        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['self']['is_ready'], True)
        self.assertEqual(j['players'][0]['is_ready'], True)
        self.assertEqual(j['players'][1]['is_ready'], True)
        self.assertEqual(j['players'][2]['is_ready'], False)
        self.assertEqual(len(j['players']), 3)
        self.assertEqual(j['phase'], 'wait-for-ready') # must not be submit-word yet!

        p = '/games/%s/players/%s/ready' % (self.game_token, self.player_token[2])
        r = requests.put(self.api(p))
        self.assertEqual(r.status_code, 200)

        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['self']['is_ready'], True)
        self.assertEqual(j['players'][0]['is_ready'], True)
        self.assertEqual(j['players'][1]['is_ready'], True)
        self.assertEqual(j['players'][2]['is_ready'], True)
        self.assertEqual(len(j['players']), 3)
        self.assertEqual(j['phase'], 'submit-word')

    def test_player_join(self):
        self.test_new_game()
        p = '/games/%s/players' % self.game_token
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        players = r.json()['players']
        self.assertEqual(players, ['Player 1'])

        r = requests.post(self.api(p), json={
            'player_name': 'Player 3',
        })
        self.assertEqual(r.status_code, 201)
        self.player_token[2] = r.json()['player_token']

        r = requests.post(self.api(p), json={
            'player_name': 'Player 2',
        })
        self.assertEqual(r.status_code, 201)
        self.player_token[1] = r.json()['player_token']

        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        players = r.json()['players']
        self.assertEqual(players, ['Player 1', 'Player 2', 'Player 3'])

    def test_player_join_same_name(self):
        self.test_new_game()
        p = '/games/%s/players' % self.game_token
        r = requests.post(self.api(p), json={
            'player_name': 'Player 2',
        })
        self.assertEqual(r.status_code, 201)
        p = '/games/%s/players' % self.game_token
        r = requests.post(self.api(p), json={
            'player_name': 'Player 2',
        })
        self.assertEqual(r.status_code, 400)

    def test_player_join_long_name(self):
        self.test_new_game()
        p = '/games/%s/players' % self.game_token
        r = requests.post(self.api(p), json={
            'player_name': 'a'*17,
        })
        self.assertEqual(r.status_code, 400)

    def test_player_join_short_name(self):
        self.test_new_game()
        p = '/games/%s/players' % self.game_token
        r = requests.post(self.api(p), json={
            'player_name': 'a',
        })
        self.assertEqual(r.status_code, 400)

    def test_player_join_bad_start(self):
        self.test_new_game()
        p = '/games/%s/players' % self.game_token
        r = requests.post(self.api(p), json={
            'player_name': ' Foo',
        })
        self.assertEqual(r.status_code, 400)

    def test_player_join_bad_end(self):
        self.test_new_game()
        p = '/games/%s/players' % self.game_token
        r = requests.post(self.api(p), json={
            'player_name': 'Foo ',
        })
        self.assertEqual(r.status_code, 400)

    def test_player_list_invalid_game_token(self):
        p = '/games/%s/players' % '123'
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 404)

    def test_zero_score(self):
        self.test_player_join()
        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(len(j['scoreboard_order']), 3)
        for x in range(3):
            self.assertEqual(j['players'][x]['score_total'], 0)
            self.assertEqual(j['players'][x]['score_own_words'], 0)
            self.assertEqual(j['players'][x]['score_correct_guesses'], 0)
            self.assertTrue(j['players'][x]['id'] in j['scoreboard_order'])

    def test_player_ready(self):
        self.test_player_join()

        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['players'][0]['name'], 'Player 1')
        self.assertEqual(j['players'][1]['name'], 'Player 2')
        self.assertEqual(j['players'][2]['name'], 'Player 3')
        self.assertEqual(j['self']['is_ready'], False)
        self.assertEqual(j['players'][0]['is_ready'], False)
        self.assertEqual(j['players'][1]['is_ready'], False)
        self.assertEqual(j['players'][2]['is_ready'], False)
        p = '/games/%s/players/%s/ready' % (self.game_token, self.player_token[0])
        r = requests.put(self.api(p))
        self.assertEqual(r.status_code, 200)

        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['self']['is_ready'], True)
        self.assertEqual(j['players'][0]['is_ready'], True)
        self.assertEqual(j['players'][1]['is_ready'], False)
        self.assertEqual(j['players'][2]['is_ready'], False)
        self.assertEqual(j['players'][0]['is_self'], True)
        self.assertEqual(j['players'][1]['is_self'], False)
        self.assertEqual(j['players'][2]['is_self'], False)

    def test_game_start(self):
        self.test_player_ready()
        p = '/games/%s/players/%s/ready' % (self.game_token, self.player_token[1])
        r = requests.put(self.api(p))
        self.assertEqual(r.status_code, 200)

        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['phase'], 'wait-for-ready')

        p = '/games/%s/players/%s/ready' % (self.game_token, self.player_token[2])
        r = requests.put(self.api(p))
        self.assertEqual(r.status_code, 200)

        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['phase'], 'submit-word')
        for x in range(3):
            self.assertEqual(j['players'][x]['name'], 'Player %d' % (x+1))
            self.assertEqual(j['players'][x]['is_ready'], True)
            self.assertTrue(isinstance(j['players'][x]['id'], int))
            self.assertEqual(len(j['players'][x]['letters']), 12)

        # random, no further check
        self.assertTrue(len(j['self']['card']['text']) > 4)
        self.assertEqual(j['players'][0]['letters'], j['self']['letters'])

        self.assertEqual(j['round'], 1)

    def test_no_join_after_game_start(self):
        self.test_game_start()
        p = '/games/%s/players' % self.game_token
        r = requests.post(self.api(p), json={
            'player_name': 'Player 4',
        })
        self.assertEqual(r.status_code, 403)

    def test_multiple_ready_requests(self):
        self.test_game_start()
        p = '/games/%s/players/%s/ready' % (self.game_token, self.player_token[0])
        r = requests.put(self.api(p))
        self.assertEqual(r.status_code, 403)

    def test_submit_word(self):
        self.test_game_start()

        for loop, x in enumerate(self.player_token):
            p = '/games/%s/players/%s' % (self.game_token, self.player_token[x])
            r = requests.get(self.api(p))
            self.assertEqual(r.status_code, 200)
            j = r.json()
            self.assertEqual(j['phase'], 'submit-word')

            word = j['self']['letters'][1:4]
            if loop == 0:
                word = j['self']['letters']
            p = '/games/%s/players/%s/word' % (self.game_token, self.player_token[x])
            r = requests.put(self.api(p), json={
                'word': word,
            })
            self.assertEqual(r.status_code, 200)

            p = '/games/%s/players/%s' % (self.game_token, self.player_token[x])
            r = requests.get(self.api(p))
            self.assertEqual(r.status_code, 200)
            j = r.json()
            self.assertEqual(j['self']['word'], word)
            for player in j['players']:
                if player['is_self']:
                    self.assertEqual(player['word'], word)

    def test_submit_word_too_long(self):
        self.test_game_start()

        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['phase'], 'submit-word')

        p = '/games/%s/players/%s/word' % (self.game_token, self.player_token[0])
        r = requests.put(self.api(p), json={
            'word': j['self']['letters'] + j['self']['letters'][0],
        })
        self.assertEqual(r.status_code, 400)

    def test_submit_word_invalid_letter(self):
        self.test_game_start()

        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['phase'], 'submit-word')

        word = 'A'
        for word in string.ascii_uppercase:
            if word not in j['self']['letters']:
                # found an invalid letter -> use it.
                break

        p = '/games/%s/players/%s/word' % (self.game_token, self.player_token[0])
        r = requests.put(self.api(p), json={
            'word': word
        })
        self.assertEqual(r.status_code, 400)

    def test_assign_all_words(self):
        self.test_submit_word()
        for loop, player_token in enumerate(self.player_token.values()):
            p = '/games/%s/players/%s' % (self.game_token, player_token)
            r = requests.get(self.api(p))
            self.assertEqual(r.status_code, 200)
            j = r.json()
            self.assertEqual(j['phase'], 'assign-words')
            players_with_all_words_assigned = 0
            for player in j['players']:
                if player['all_words_assigned']:
                    players_with_all_words_assigned += 1
            self.assertEqual(players_with_all_words_assigned, loop)
            self.assertEqual(len(j['cards']), 3+3)
            usable_card_ids = []
            for card in j['cards']:
                if card['is_self']:
                    continue
                usable_card_ids.append(card['id'])
            guesses = {}
            for player in j['players']:
                if player['is_self']:
                    continue
                guesses[str(player['id'])] = usable_card_ids.pop(0)
            p = '/games/%s/players/%s/guesses' % (self.game_token, player_token)
            r = requests.put(self.api(p), json={'guesses': guesses})
            self.assertEqual(r.status_code, 200)
            p = '/games/%s/players/%s/guesses' % (self.game_token, player_token)
            r = requests.get(self.api(p))
            if player_token == list(self.player_token.values())[-1]:
                # phase will already be 'score' which no longer permits
                # this call.
                self.assertEqual(r.status_code, 403)
            else:
                self.assertEqual(r.status_code, 200)
                j = r.json()
                self.assertEqual(j['guesses'], guesses)
        for player_token in self.player_token.values():
            p = '/games/%s/players/%s' % (self.game_token, player_token)
            r = requests.get(self.api(p))
            self.assertEqual(r.status_code, 200)
            j = r.json()
            self.assertEqual(j['phase'], 'score')
            for player in j['players']:
                self.assertEqual(player['all_words_assigned'], True)

    def test_assign_words_fail_on_missing_player(self):
        self.test_submit_word()
        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        usable_card_ids = []
        for card in j['cards']:
            if card['is_self']:
                continue
            usable_card_ids.append(card['id'])
        guesses = {}
        for player in j['players']:
            if player['is_self']:
                continue
            guesses[str(player['id'])] = usable_card_ids.pop(0)
            # Exit after having populated just one player instead of two:
            break
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.put(self.api(p), json={'guesses': guesses})
        self.assertEqual(r.status_code, 400)
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['guesses'], {})

    def test_assign_words_reject_own(self):
        self.test_submit_word()
        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        usable_card_ids = []
        for card in j['cards']:
            if card['is_self']:
                continue
            usable_card_ids.append(card['id'])
        guesses = {}
        for player in j['players'][0:2]:
            guesses[str(player['id'])] = usable_card_ids.pop(0)
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.put(self.api(p), json={'guesses': guesses})
        self.assertEqual(r.status_code, 403)
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['guesses'], {})

    def test_assign_words_reject_bad_player_id(self):
        self.test_submit_word()
        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['phase'], 'assign-words')
        self.assertEqual(len(j['cards']), 3+3)
        usable_card_ids = []
        for card in j['cards']:
            if card['is_self']:
                continue
            usable_card_ids.append(card['id'])
        guesses = {}
        for idx, player in enumerate(j['players']):
            if player['is_self']:
                continue
            guesses[str(idx+1)] = usable_card_ids.pop(0)
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.put(self.api(p), json={'guesses': guesses})
        self.assertEqual(r.status_code, 400)
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['guesses'], {})

    def test_assign_words_reject_bad_card_id(self):
        self.test_submit_word()
        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['phase'], 'assign-words')
        self.assertEqual(len(j['cards']), 3+3)
        usable_card_ids = [1, 2, 3]
        guesses = {}
        for player in j['players']:
            if player['is_self']:
                continue
            guesses[str(player['id'])] = usable_card_ids.pop(0)
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.put(self.api(p), json={'guesses': guesses})
        self.assertEqual(r.status_code, 400)
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['guesses'], {})

    def test_assign_words_reject_duplicate_card_id(self):
        self.test_submit_word()
        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['phase'], 'assign-words')
        self.assertEqual(len(j['cards']), 3+3)
        usable_card_ids = []
        for card in j['cards']:
            if card['is_self']:
                continue
            usable_card_ids.append(card['id'])
        guesses = {}
        for player in j['players']:
            if player['is_self']:
                continue
            guesses[str(player['id'])] = usable_card_ids[0]
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.put(self.api(p), json={'guesses': guesses})
        self.assertEqual(r.status_code, 400)
        p = '/games/%s/players/%s/guesses' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['guesses'], {})

    def test_score(self):
        self.test_assign_all_words()
        p = '/games/%s/players/%s' % (self.game_token, self.player_token[0])
        r = requests.get(self.api(p))
        self.assertEqual(r.status_code, 200)
        j = r.json()
        self.assertEqual(j['phase'], 'score')
        self.assertTrue(isinstance(j['currently_scored']['word'], str))
        # Test words are always 3 or 12 (all letters) letters long:
        self.assertTrue(len(j['currently_scored']['word']) in (3, 12))
        self.assertTrue(isinstance(j['currently_scored']['player_id'], int))
        self.assertEqual(len(list(j['currently_scored']['guesses'].keys())), 2)
        self.assertTrue(isinstance(list(j['currently_scored']['guesses'].keys())[0], str))
        self.assertTrue(isinstance(list(j['currently_scored']['guesses'].keys())[1], str))
        self.assertTrue(isinstance(list(j['currently_scored']['guesses'].values())[0], int))
        self.assertTrue(isinstance(list(j['currently_scored']['guesses'].values())[1], int))
        self.assertEqual(len(j['cards']), 6)
        for card in j['cards']:
            self.assertTrue('score' in card)
            if card['is_self']:
                self.assertNotEqual(card.get('player_id'), None)
            else:
                self.assertEqual(card.get('player_id'), None)

        while j['phase'] == 'score':
            for x in range(3):
                j_prev = j
                p = '/games/%s/players/%s' % (self.game_token, self.player_token[x])
                r = requests.get(self.api(p))
                self.assertEqual(r.status_code, 200)
                j = r.json()
                own_player_id = 0
                for player in j['players']:
                    if player['is_self']:
                        own_player_id = player['id']
                        break
                p = '/games/%s/players/%s/scored' % (self.game_token, self.player_token[x])
                r = requests.put(self.api(p))
                if j['currently_scored']['player_id'] != own_player_id:
                    # we don't know who is allowed to reveal first, so
                    # we brute force...
                    self.assertEqual(r.status_code, 403)
                    continue
                self.assertEqual(r.status_code, 200)

                p = '/games/%s/players/%s' % (self.game_token, self.player_token[x])
                r = requests.get(self.api(p))
                self.assertEqual(r.status_code, 200)
                j = r.json()
                if x != 2:
                    self.assertNotEqual(j['currently_scored']['word'], j_prev['currently_scored']['word'])
                    self.assertNotEqual(j['currently_scored']['player_id'], j_prev['currently_scored']['player_id'])
                    self.assertNotEqual(j['currently_scored']['guesses'], j_prev['currently_scored']['guesses'])
        revealed_cards = 0
        for card in j['cards']:
            if card.get('player_id'):
                revealed_cards += 1
        self.assertEqual(revealed_cards, 3)
        self.assertEqual(j['phase'], 'wait-for-ready')


if __name__ == '__main__':
    sys.stdout.write("Waiting for webserver to become responsive")
    for x in range(2000):
        try:
            r = requests.get(SERVER)
            sys.stdout.write('\n')
            break
        except requests.exceptions.ConnectionError:
            sys.stdout.write('.')
            sys.stdout.flush()
            time.sleep(0.1)
    unittest.main()
