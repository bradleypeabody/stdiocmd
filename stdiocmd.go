// Provide a server interface on top of JSON (or other) encoding using raw streams, with support for command line stdin/stdio.
// This package was created for us with Go servers that run inside Electron apps, so you
// can easily have communication back and forth to negotate starting a webserver, secret
// keys/cookies or anything else that might need to occur prior to just using Electron to
// hit your Go webserver.
package stdiocmd

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"runtime/debug"
	"sync"
)

// MessageWriter is able to write a message to the underlying output.
type MessageWriter interface {
	WriteMessage(m Message) error
}

// EncoderMessageWriter implements MessageWriter on top of an Encoder.
type EncoderMessageWriter struct {
	Encoder Encoder
}

func (emw *EncoderMessageWriter) WriteMessage(m Message) error {
	return emw.Encoder.Encode(m)
}

// Decoder is something that can decode values, same as json.Decoder.
type Decoder interface {
	Decode(v interface{}) error
}

// Encoder is something that can encode values, same as json.Encoder.
type Encoder interface {
	Encode(v interface{}) error
}

// SyncEncoder wraps an Encoder with a lock so only one Encode call happens at a time.
type SyncEncoder struct {
	Encoder Encoder
	mu      sync.Mutex
}

func (se *SyncEncoder) Encode(v interface{}) error {
	se.mu.Lock()
	defer se.mu.Unlock()
	return se.Encoder.Encode(v)
}

// Message is a generic string-keyed map.
type Message map[string]interface{}

// MessageHandler is implemented by the user of this package to provide behavior when a Message is received.
type MessageHandler interface {
	HandleMessage(w MessageWriter, m Message)
}

// MessageHandlerFunc adapts a function as a MessageHandler.  Same concept as http.HandlerFunc.
type MessageHandlerFunc func(w MessageWriter, m Message)

func (f MessageHandlerFunc) HandleMessage(w MessageWriter, m Message) {
	f(w, m)
}

// NewStdMessageServer returns a Message server connected to stdin and stdout.
func NewStdMessageServer(h MessageHandler) *MessageServer {
	return &MessageServer{
		// decode from stdin
		InDecoder: json.NewDecoder(os.Stdin),
		// wrap stdout with a SyncEncoder to ensure multiple writes don't interleave.
		OutEncoder: &SyncEncoder{Encoder: json.NewEncoder(os.Stdout)},
		//
		MessageHandler: h,
	}
}

// MessageServer implements a Serve loop on top of an Encoder and Decoder and uses a MessageHandler to process messages.
type MessageServer struct {
	InDecoder      Decoder
	OutEncoder     Encoder
	MessageHandler MessageHandler
	wg             sync.WaitGroup
}

// Wait blocks until all goroutines started by Serve() have exited.
func (ms *MessageServer) Wait() {
	ms.wg.Wait()
}

// Serve will run the MessageServer until either InDecoder or OutDecoder return an unrecoverable error.
func (ms *MessageServer) Serve() (reterr error) {

	for {
		mw := &EncoderMessageWriter{Encoder: ms.OutEncoder}
		msg := make(Message)
		err := ms.InDecoder.Decode(&msg)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			reterr = err
			break
		} else if err != nil {
			log.Printf("MessageServer.Serve() got error while decoding input: %v", err)
			continue
		}
		ms.wg.Add(1)
		go func(mh MessageHandler, mw MessageWriter, msg Message) {
			defer ms.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					st := debug.Stack()
					log.Printf("Caught panic in MessageServer.Serve(): %v\n%s", r, st)
				}
			}()
			mh.HandleMessage(mw, msg)
		}(ms.MessageHandler, mw, msg)
	}

	return
}
