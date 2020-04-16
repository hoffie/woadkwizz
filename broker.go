package main

import (
	"time"
)

type brokerChannel struct {
	id      uint64
	channel chan string
}

type brokerMessage struct {
	id      uint64
	message string
}

type Broker struct {
	clients        map[uint64]map[chan string]bool
	events         chan brokerMessage
	newClients     chan brokerChannel
	closingClients chan brokerChannel
}

func NewBroker() *Broker {
	b := &Broker{
		events:         make(chan brokerMessage, 10),
		newClients:     make(chan brokerChannel),
		closingClients: make(chan brokerChannel),
		clients:        make(map[uint64]map[chan string]bool),
	}

	// Listen for events to distribute:
	go b.listen()

	return b
}

// Listen on different channels and act accordingly
func (b *Broker) listen() {
	wait := time.Second * 1
	removeClient := func(id uint64, channel chan string) {
		if _, exists := b.clients[id]; !exists {
			return
		}
		if _, exists := b.clients[id][channel]; !exists {
			return
		}
		delete(b.clients[id], channel)
		if len(b.clients[id]) == 0 {
			delete(b.clients, id)
		}
		close(channel)
	}
	for {
		select {
		case s := <-b.newClients:
			// A new client has connected.
			// Register their message channel
			if _, exists := b.clients[s.id]; !exists {
				b.clients[s.id] = make(map[chan string]bool)
			}
			b.clients[s.id][s.channel] = true

		case s := <-b.closingClients:
			// A client has detached and we want to
			// stop sending them messages.
			removeClient(s.id, s.channel)

		case event := <-b.events:
			// We got a new event from the outside!
			// Send event to all connected clients
			chans, exists := b.clients[event.id]
			if !exists {
				// an event for an id we don't know (maybe there are no live listeners).
				// just ignore it:
				continue
			}
			for clientChan, _ := range chans {
				select {
				case clientChan <- event.message:
				case <-time.After(wait):
					removeClient(event.id, clientChan)
				}
			}
		}
	}
}

func (b *Broker) NewClientChan(id uint64) chan string {
	c := brokerChannel{
		id:      id,
		channel: make(chan string),
	}
	b.newClients <- c
	return c.channel
}

func (b *Broker) RemoveClientChan(id uint64, channel chan string) {
	c := brokerChannel{
		id:      id,
		channel: channel,
	}
	b.closingClients <- c
}

func (b *Broker) Send(id uint64, message string) {
	c := brokerMessage{
		id:      id,
		message: message,
	}
	b.events <- c
}
