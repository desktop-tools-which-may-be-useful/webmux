package hub

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
	"github.com/rainchan/webmux/internal/contract"
	"github.com/rainchan/webmux/internal/voice"
	"github.com/rainchan/webmux/internal/wire"
)

// Voice bridges one speech recognition session to a WebSocket connection.
// Binary frames from the client are written as raw 16kHz mono s16le PCM to the
// subprocess STDIN; a {"type":"stop"} text frame closes STDIN so the
// subprocess flushes its final transcription.
func Voice(ctx context.Context, sess *voice.Session, finishTimeout time.Duration, c *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	c.SetReadLimit(65536)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			typ, data, err := c.Read(ctx)
			if err != nil {
				cancel()
				return
			}
			switch typ {
			case websocket.MessageBinary:
				if err := sess.WriteAudio(data); err != nil {
					sess.StopInput(finishTimeout)
				}
			case websocket.MessageText:
				var msg contract.VoiceClientMessage
				if json.Unmarshal(data, &msg) == nil && msg.Type == "stop" {
					sess.StopInput(finishTimeout)
				}
			}
		}
	}()
	defer func() {
		cancel()
		sess.Close()
		_ = c.CloseNow()
		<-readerDone
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-sess.Events():
			if !ok {
				_ = c.Close(websocket.StatusNormalClosure, "done")
				return
			}
			data, err := json.Marshal(wire.VoiceEvent(ev))
			if err != nil {
				return
			}
			if write(ctx, c, websocket.MessageText, data) != nil {
				_ = c.Close(4408, "write timeout")
				return
			}
			if ev.Type == voice.EventFinal || ev.Type == voice.EventError {
				_ = c.Close(websocket.StatusNormalClosure, "done")
				return
			}
		}
	}
}
