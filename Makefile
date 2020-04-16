all: build test
build: *.go cards.b64
	gofmt -w *.go
	go build

test: build
	bash -c 'echo $$$$ > test.pid; exec ./woadkwizz -debug' &
	./test.py -vf; bash -c 'kill $$(<test.pid)'
	rm -f test.pid

debug-run: build
	./woadkwizz -debug

edit-cards:
	base64 -d < cards.b64 > cards.txt
	$$EDITOR cards.txt
	base64 < cards.txt > cards.b64
	rm cards.txt

.PHONY: all build test edit-cards
