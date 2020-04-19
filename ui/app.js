async function GET(url = '') {
  App.log("debug", "GET " + url);
  return await handleError(async function() {
    const response = await fetch(url, {
      method: 'GET',
      mode: 'same-origin',
      cache: 'no-cache',
      credentials: 'omit',
      referrerPolicy: 'no-referrer',
    });
    return await response;
  });
}

async function POST(url = '', data = {}) {
  App.log("debug", "POST " + url + ": " + data);
  return await handleError(async function() {
    const response = await fetch(url, {
      method: 'POST',
      mode: 'same-origin',
      cache: 'no-cache',
      credentials: 'omit',
      headers: {
        'Content-Type': 'application/json'
      },
      referrerPolicy: 'no-referrer',
      body: JSON.stringify(data)
    });
    return await response;
  });
}

async function PUT(url = '', data = {}) {
  App.log("debug", "PUT " + url + ": " + data);
  return await handleError(async function() {
    const response = await fetch(url, {
      method: 'PUT',
      mode: 'same-origin',
      cache: 'no-cache',
      credentials: 'omit',
      headers: {
        'Content-Type': 'application/json'
      },
      referrerPolicy: 'no-referrer',
      body: JSON.stringify(data)
    });
    return await response;
  });
}

async function handleError(f) {
  try {
    var response = await f();
  } catch(e) {
    App.log("error", "API error: " + e);
    console.log(e);
    throw e;
  }

  if (!response.ok) {
    App.log("error", "API http status: " + response.status);
    console.log(response);
    throw "api call failed";
  }

  return response.json();
}

const NewGame = {
  template: '#new-game-template',
  data: function() {
    return {
      'playerName': '',
    }
  },
  methods: {
    newGame: function(event) {
      if (!this.playerName) return;
      POST('/api/games', {
        'player_name': this.playerName
      }).then((d) => {
        this.$router.push({
          'name': 'Board',
          'params': {
            'game_token': d.game_token,
            'player_token': d.player_token,
          },
        });
      });
    },
    focusInput: function() {
      this.$refs.playerName.$el.focus();
    },
  },
  mounted: function() {
    this.focusInput();
  },
}
const JoinGame = {
  template: '#join-game-template',
  data: function() {
    return {
      'playerName': '',
      'players': [],
    }
  },
  methods: {
    joinGame: function(event) {
      if (!this.playerName) return;
      POST('/api/games/' + this.$route.params.game_token + '/players', {
        'player_name': this.playerName,
      }).then((d) => {
        this.$router.push({
          'name': 'Board',
          'params': {
            'game_token': this.$route.params.game_token,
            'player_token': d.player_token,
          },
        });
      });
    },
    fetch: function() {
      GET('/api/games/' + this.$route.params.game_token + '/players').then((d) => {
        this.players = d.players;
      });
    },
    focusInput: function() {
      this.$refs.playerName.$el.focus();
    },
  },
  mounted: function() {
    this.focusInput();
    this.fetch();
    this.$nextTick(function() {
      App.addEventListener('players', this.fetch);
    });
  },
}

Vue.component('player', {
  template: '#player-template',
  props: ['player', 'board', 'guesses'],
  data: function() {
    return {
      'cards': this.player.is_self ? [this.board.self.card] : [],
    };
  },
  computed: {
    'correct_card': function() {
      for (var i = 0; i < this.board.cards.length; i++) {
        var card = this.board.cards[i];
        if (card.player_id == this.player.id) {
          return card;
        }
      }
      return {};
    },
  },
  methods: {
    markReady: function(event) {
      PUT('/api/games/' + this.$route.params.game_token + '/players/' + this.$route.params.player_token + '/ready', {});
    },
    onCardAdd: function(event) {
      this.$emit('guess-word', {
        player_id: this.player.id,
        card_id: this.cards[0].id,
      });
    },
    onCardRemove: function(event) {
      this.$emit('guess-word', {
        player_id: this.player.id,
        card_id: null,
      });
    },
  },
  watch: {
    'player': function() {
      if (!this.player.is_self) return;
      this.cards = [this.board.self.card];
      return;
    },
    'guesses': function() {
      if (this.player.is_self) return;
      if (!(this.player.id in this.guesses)) return;
      for (var i = 0; i < this.board.cards.length; i++) {
        var card = this.board.cards[i];
        if (card.id == this.guesses[this.player.id]) {
          if (this.cards == [card]) break;
          this.cards = [card];
          break;
        }
      }
    },
    'board.round': function() {
      this.cards = [];
    },
  },
});

Vue.component('middle-card', {
  template: '#middle-card-template',
  props: ['text', 'hover', 'score'],
});

const Board = {
  template: '#board-template',
  props: ['scoreboardButtonPressed'],
  computed: {
    currently_scored_player: function() {
      var player_id = this.board.currently_scored.player_id;
      var player = this.getPlayer(player_id);
      return player || {};
    },
    currently_scored_guesses: function() {
      var players_by_card = {};
      for (var player_id in this.board.currently_scored.guesses) {
        var card_id = this.board.currently_scored.guesses[player_id];
        var card = this.getCard(card_id);
        if (!(card_id in players_by_card)) {
          players_by_card[card_id] = {
            card: card,
            players: [],
          };
        }
        var player = this.getPlayer(player_id);
        players_by_card[card_id].players.push(player);
      }
      var cards_with_players = [];
      for (var card_id in players_by_card) {
        cards_with_players.push(players_by_card[card_id]);
      }
      cards_with_players.sort(function(a, b) {
        return a.players.length < b.players.length ? -1 : 1;
      });
      return cards_with_players;
    },
  },
  data: function() {
    return {
      'baseURL': window.location.protocol + '//' + window.location.host,
      'board': {
        'self': {
          'letters': '',
        },
        'cards': [],
        'players': [],
      },
      'show_currently_scored': false,
      'letters': [],
      'word_letters': [],
      'guesses': {},
      'num_guesses': 0,
      'guesses_saved': false,
      'middle_cards': [],
      'scoreboard': [],
      'scoreboardVisible': false,
      'helpSnackbarsEnabled': true,
    }
  },
  methods: {
    getCard: function(id) {
      for (var i = 0; i < this.board.cards.length; i++) {
        var card = this.board.cards[i];
        if (card.id == id) {
          return card;
        }
      }
    },
    getPlayer: function(id) {
      // TODO performance: should be map-based lookup
      for (var i = 0; i < this.board.players.length; i++) {
        var player = this.board.players[i];
        if (player.id == id) {
          return player;
        }
      }
    },
    updateScoreboard: function() {
      var new_scoreboard = [];
      for (var i = 0; i < this.board.scoreboard_order.length; i++) {
        var player_id = this.board.scoreboard_order[i];
        var player = this.getPlayer(player_id);
        new_scoreboard.push({
          name: player.name,
          score_own_words: player.score_own_words,
          score_correct_guesses: player.score_correct_guesses,
          score_total: player.score_total,
        });
      }
      this.scoreboard = new_scoreboard;
    },
    fetch: function() {
      if (!this.$route.params.player_token) return;
      GET('/api/games/' + this.$route.params.game_token + '/players/' + this.$route.params.player_token).then((d) => {
        if (d.phase == 'submit-word' && this.board.phase != 'submit-word') {
          this.word_letters = d.self.word.split('');
          this.letters = d.self.letters.split('');
          this.num_guesses = 0;
          this.guesses_saved = false;
        }

        var animTime = 0;
        if (this.board.phase == 'score' && this.show_currently_scored) {
          // longer animation
          animTime = 2000;
        }
        // must be called before the board is replaced:
        if (this.board.phase == 'score' && d.phase != 'score') {
          this.show_currently_scored = false;
          // wait for animation to finish, then switch board,
          // update scoreboard and display it.
          window.setTimeout(function() {
            this.board = d;
            this.updateScoreboard();
            this.scoreboardVisible = true;
          }.bind(this), animTime);
          // end early to avoid updating the board here:
          return;
        }

        if (this.board.currently_scored != d.currently_scored) {
          this.show_currently_scored = false;
          window.setTimeout(function() {
            this.show_currently_scored = true;
          }.bind(this), animTime);
        }

        this.board = d;

        // must be called after the new board is set:
        this.updateScoreboard();

        if (this.board.phase == 'assign-words') {
          GET('/api/games/' + this.$route.params.game_token + '/players/' + this.$route.params.player_token + '/guesses').then((g) => {
            if (this.num_guesses != 0) {
              // Do not overwrite unsaved guesses / only set guesses on initial load.
              return;
            }
            this.guesses = g['guesses'];
            var guessed_card_ids = [];
            for (var player_id in this.guesses) {
              guessed_card_ids.push(this.guesses[player_id]);
              this.num_guesses++;
              this.guesses_saved = true;
            }
            var new_middle_cards = [];
            for (var i = 0; i < this.board.cards.length; i++) {
              var card = this.board.cards[i];
              if (guessed_card_ids.indexOf(card.id) === -1) {
                new_middle_cards.push(card);
              }
            }
            this.middle_cards = new_middle_cards;
          });
        }
      });
    },
    submitWord: function() {
      if (!this.word_letters.length) return;
      PUT('/api/games/' + this.$route.params.game_token + '/players/' + this.$route.params.player_token + '/word', {
        word: this.word_letters.join(''),
      });
    },
    markScored: function() {
      PUT('/api/games/' + this.$route.params.game_token + '/players/' + this.$route.params.player_token + '/scored');
    },
    onGuess: function(guess) {
      this.guesses[guess.player_id] = guess.card_id;
      if (guess.card_id === null) {
        this.num_guesses--;
      } else {
        this.num_guesses++;
      }
    },
    submitGuesses: function() {
      PUT('/api/games/' + this.$route.params.game_token + '/players/' + this.$route.params.player_token + '/guesses', {guesses: this.guesses}).then((d) => {
        this.guesses_saved = true;
      });
    },
  },
  mounted: function() {
    this.$nextTick(function() {
      App.setupEventSource(this.$route.params.game_token);
      App.addEventListener('players', this.fetch);
      App.addEventListener('board', this.fetch);
    });
  },
  watch: {
    'scoreboardVisible': function() {
      if (!this.scoreboardVisible) {
        this.$emit('scoreboard-closed');
      }
    },
    'scoreboardButtonPressed': function(next, prev) {
      if (next == true) {
        this.scoreboardVisible = true;
      }
    },
  },
}

const routes = [
  { name: 'NewGame', path: '/', component: NewGame },
  { name: 'JoinGame', path: '/games/:game_token', component: JoinGame },
  { name: 'Board', path: '/games/:game_token/players/:player_token', component: Board }
]

const router = new VueRouter({
  mode: "history",
  routes
})

var App = {
  eventSource: null,
  game_token: null,
  eventSourceReconnectTime: 0.2,
  setupEventSource: function(game_token) {
    if (!game_token) return;
    if (App.eventSource && App.game_token == game_token) return;
    App.game_token = game_token;
    App.eventSource = new EventSource('/api/games/' + game_token + '/events');
    App.eventSource.onerror = function(err) {
      App.eventSource.close();
      console.log("eventSource failed:", err);
      App.eventSource = null;
      App.eventListenerRequestsIdx = 0;
      if (App.eventSourceReconnectTime < 32) {
        App.eventSourceReconnectTime *= 2;
      }
      console.log("scheduling eventSource reconnect in seconds:", App.eventSourceReconnectTime);
      window.setTimeout(function() {
        console.log("reconnecting eventSource");
        App.setupEventSource(game_token);
      }, App.eventSourceReconnectTime*1000);
    };
    App.eventSource.onopen = function() {
      App.eventSourceReconnectTime = 1;
    };
    App.runEventListenerRequests();
  },
  eventListenerRequests: [],
  eventListenerRequestsIdx: 00,
  addEventListener: function(event, callback) {
    App.eventListenerRequests.push([event, callback]);
    App.runEventListenerRequests();
  },
  runEventListenerRequests: function() {
    if (!App.eventSource) return;
    for (; App.eventListenerRequestsIdx < App.eventListenerRequests.length; App.eventListenerRequestsIdx++) {
      var e = App.eventListenerRequests[App.eventListenerRequestsIdx];
      App.eventSource.addEventListener(e[0], e[1]);
      console.log("registering and running eventSource listener", e);
      e[1]();
    }
  },
  log: function(level, message) {
    if (level == "error") {
      alert(message);
    }
  },
};

Vue.use(VueMaterial.default);

const app = new Vue({
  router,
  template: '#app-template',
  data: function() {
    return {
      aboutDialogVisible: false,
      scoreboardButtonPressed: false,
      consoleVisible: false,
      errorCount: 0,
      logMessages: [],
    };
  },
  mounted: function() {
    App.log = this.log;
    App.setupEventSource(this.$route.params.game_token);
  },
  watch: {
    '$route': function(to, from) {
      App.setupEventSource(this.$route.params.game_token);
    },
  },
  methods: {
    log: function(level, message) {
      if (level == "error") {
        this.errorCount++;
      }
      this.logMessages.unshift({
        time: new Date(),
        level: level,
        message: message,
      });
    },
  },
}).$mount('#app');
