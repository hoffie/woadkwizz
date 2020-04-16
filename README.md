A browser-based creativity game for three or more players.
You are tasked to describe a randomly assigned thing using a newly invented word.
You gain points when you guess other players' words or other players guess yours corectly.

## Requirements for players
A modern browser (HTML5/JavaScript/CSS) with network connectivity to the server. Tested with Firefox 75 and Chromium 81.

## Status
This project is in a working state.
No major additions are currently planned.

## Limitations
- No high-end graphics. This is a simple, textual browser-based game.
- German-only. In case someone wants to add support for multi-language support, pull requests will be accepted.

## Tech stack
The backend is written in [Golang](https://golang.org), using [Gin](https://github.com/gin-gonic/gin) as web framework and [gorm](https://github.com/jinzhu/gorm) as object-relational mapper with an [SQLite](https://sqlite.org) backend.
Tests are run via [Python 3](https://python.org).
The frontend uses [Vue](https://vuejs.org) with [Vue-Router](https://router.vuejs.org) and [Vue-Material](https://vuematerial.io).
Drag'n'Drop support is provided by [SortableJS](https://github.com/SortableJS/Sortable)/[Vue.Draggable](https://github.com/SortableJS/Vue.Draggable).

### Getting started
`git clone https://github.com/hoffie/woadkwizz && cd woadkwizz && make debug-run`

### Run tests
`make test`

## License
This implementation is licensed under [AGPLv3](LICENSE.AGPLv3).

## Author
WoadKwizz was implemented by [Christian Hoffmann](https://hoffmann-christian.info) in 2020.
