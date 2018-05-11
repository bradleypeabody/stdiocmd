package stdiocmd

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"testing"
)

func TestMessageServer(t *testing.T) {

	var inMessages []Message

	n := 0
	h := MessageHandlerFunc(func(w MessageWriter, m Message) {
		t.Logf("GOT MESSAGE: %+v", m)
		inMessages = append(inMessages, m)
		reply := make(Message)
		reply["is_reply"] = n
		n++
		w.WriteMessage(reply)
		t.Logf("SENDING REPLY: %+v", reply)
	})

	inBytes := []byte(`{"in_test":1}` + "\n" + `{"in_test":2}` + "\n")
	var outBuf bytes.Buffer

	ms := &MessageServer{
		InDecoder:      json.NewDecoder(bytes.NewReader(inBytes)),
		OutEncoder:     json.NewEncoder(&outBuf),
		MessageHandler: h,
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := ms.Serve()
		log.Printf("Error from Serve(): %v", err)
		ms.Wait()
	}()

	wg.Wait()

	log.Printf("outBuf: %s", outBuf.String())

	outBufStr := outBuf.String()

	if !strings.Contains(outBufStr, `"is_reply":0`) {
		t.Fatalf("outBufStr did not contain reply idx 0")
	}
	if !strings.Contains(outBufStr, `"is_reply":1`) {
		t.Fatalf("outBufStr did not contain reply idx 1")
	}

}
