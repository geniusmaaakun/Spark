package melody

import (
	"errors"
	"net/http"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
)

/*
このコードは、WebSocketベースの通信を扱うためのセッション管理を実装しています。WebSocket通信を処理するために、gorilla/websocketライブラリを利用しており、Sessionという構造体を通じて、各クライアントとの接続状態やデータ送受信を管理しています。このコードは、クライアントとサーバー間の非同期通信における接続管理を容易にするものです。
*/

// Session wrapper around websocket connections.
/*
Request: WebSocket接続が作成された際のHTTPリクエストを保持します。
Keys: セッション専用のデータストアで、任意のキーと値のペアを格納できます。
UUID: セッション固有の識別子（UUID）を保持します。
conn: WebSocket接続を表すgorilla/websocketライブラリのWebSocket接続オブジェクトです。
output: 非同期でメッセージを送信するためのチャネルです。
melody: セッションが属するMelody（WebSocket管理の上位構造体）を参照しています。
open: セッションが開かれているか（有効な接続か）を示すフラグです。
rwmutex: 読み書き時の排他制御を行うためのロック機構です。
*/
type Session struct {
	Request *http.Request
	Keys    map[string]interface{}
	UUID    string
	conn    *ws.Conn
	output  chan *envelope
	melody  *Melody
	open    bool
	rwmutex *sync.RWMutex
}

//writeMessage: メッセージをセッションに非同期で書き込みます。outputチャネルにメッセージを送信することで、非同期のメッセージ送信を行います。
func (s *Session) writeMessage(message *envelope) {
	//closed(): セッションが閉じているかを確認し、閉じていればエラーハンドラーを呼び出します。
	if s.closed() {
		s.melody.errorHandler(s, errors.New("tried to write to closed a session"))
		return
	}

	//**select**文で、outputチャネルがブロックされていないか確認し、ブロックされていない場合のみメッセージを送信します。バッファがいっぱいの場合はエラーになります。
	select {
	case s.output <- message:
		// ブロックされていたらエラー
	default:
		s.melody.errorHandler(s, errors.New("session message buffer is full"))
	}
}

//writeRaw: WebSocketのconnを使って、指定されたメッセージを直接書き込みます。
func (s *Session) writeRaw(message *envelope) error {
	if s.closed() {
		return errors.New("tried to write to a closed session")
	}

	//SetWriteDeadlineで書き込み操作にタイムアウトを設定し、WriteMessageを呼んでWebSocket経由でメッセージを送信します。
	s.conn.SetWriteDeadline(time.Now().Add(s.melody.Config.WriteWait))
	err := s.conn.WriteMessage(message.t, message.msg)

	if err != nil {
		return err
	}

	return nil
}

//closed: セッションが閉じられているかを確認します。rwmutexで排他制御し、スレッドセーフにopenの状態をチェックします。
func (s *Session) closed() bool {
	s.rwmutex.RLock()
	defer s.rwmutex.RUnlock()

	return !s.open
}

//close: セッションがまだ開いていれば、セッションを閉じます。WebSocket接続を閉じ、outputチャネルもクローズしてリソースを解放します。
func (s *Session) close() {
	if !s.closed() {
		s.rwmutex.Lock()
		s.open = false
		s.conn.Close()
		close(s.output)
		s.rwmutex.Unlock()
	}
}

//ping: WebSocket接続にPingメッセージを送信します。これにより、接続の状態を確認し、タイムアウトが発生しないように維持します。
func (s *Session) ping() {
	s.writeRaw(&envelope{t: ws.PingMessage, msg: []byte{}})
}

func (s *Session) writePump() {
	ticker := time.NewTicker(s.melody.Config.PingPeriod)
	defer ticker.Stop()

	//writePump: メッセージを処理するループです。
	//セッションのoutputチャネルからメッセージを受け取り、それをWebSocket接続に送信します。
	//また、定期的にpingメッセージを送信します。ws.CloseMessageやエラーが発生した場合はループを終了します。
loop:
	for {
		select {
		case msg, ok := <-s.output:
			if !ok {
				break loop
			}

			err := s.writeRaw(msg)

			if err != nil {
				s.melody.errorHandler(s, err)
				break loop
			}

			if msg.t == ws.CloseMessage {
				break loop
			}

			if msg.t == ws.TextMessage {
				s.melody.messageSentHandler(s, msg.msg)
			}

			if msg.t == ws.BinaryMessage {
				s.melody.messageSentHandlerBinary(s, msg.msg)
			}
		case <-ticker.C:
			s.ping()
		}
	}
}

//readPump: クライアントからのメッセージを受信し、適切なハンドラーに処理を渡すループです。
//読み込みサイズの制限やタイムアウトを設定し、Pongメッセージが来た際のハンドラーや、接続が閉じられたときの処理も設定しています。
func (s *Session) readPump() {
	s.conn.SetReadLimit(s.melody.Config.MaxMessageSize)
	s.conn.SetReadDeadline(time.Now().Add(s.melody.Config.PongWait))

	s.conn.SetPongHandler(func(string) error {
		s.conn.SetReadDeadline(time.Now().Add(s.melody.Config.PongWait))
		s.melody.pongHandler(s)
		return nil
	})

	if s.melody.closeHandler != nil {
		s.conn.SetCloseHandler(func(code int, text string) error {
			return s.melody.closeHandler(s, code, text)
		})
	}

	for {
		t, message, err := s.conn.ReadMessage()

		if err != nil {
			s.melody.errorHandler(s, err)
			break
		}

		if t == ws.TextMessage {
			s.melody.messageHandler(s, message)
		}

		if t == ws.BinaryMessage {
			s.melody.messageHandlerBinary(s, message)
		}
	}
}

//Write: テキストメッセージを書き込む関数です。非同期でメッセージを送信します。
// Write writes message to session.
func (s *Session) Write(msg []byte) error {
	if s.closed() {
		return errors.New("session is closed")
	}

	s.writeMessage(&envelope{t: ws.TextMessage, msg: msg})

	return nil
}

//WriteBinary: バイナリメッセージを書き込む関数です。
// WriteBinary writes a binary message to session.
func (s *Session) WriteBinary(msg []byte) error {
	if s.closed() {
		return errors.New("session is closed")
	}

	s.writeMessage(&envelope{t: ws.BinaryMessage, msg: msg})

	return nil
}

//Close: セッションを閉じる関数です。クローズメッセージを送信してセッションを終了します。
// Close closes session.
func (s *Session) Close() error {
	if s.closed() {
		return errors.New("session is already closed")
	}

	s.writeMessage(&envelope{t: ws.CloseMessage, msg: []byte{}})

	return nil
}

// CloseWithMsg closes the session with the provided payload.
// Use the FormatCloseMessage function to format a proper close message payload.
func (s *Session) CloseWithMsg(msg []byte) error {
	if s.closed() {
		return errors.New("session is already closed")
	}

	s.writeMessage(&envelope{t: ws.CloseMessage, msg: msg})

	return nil
}

//Set, Get, MustGet: セッション内にデータを保存・取得する関数です。セッション固有のデータを格納・取得するのに使用します。

// Set is used to store a new key/value pair exclusively for this session.
func (s *Session) Set(key string, value interface{}) bool {
	if s.closed() {
		return false
	}
	if s.Keys == nil {
		s.Keys = make(map[string]interface{})
	}

	s.Keys[key] = value
	return true
}

// Get returns the value for the given key, ie: (value, true).
// If the key does not exist, it returns (nil, false)
func (s *Session) Get(key string) (value interface{}, exists bool) {
	if s.Keys != nil {
		value, exists = s.Keys[key]
	}

	return
}

// MustGet returns the value for the given key if it exists, otherwise it panics.
func (s *Session) MustGet(key string) interface{} {
	if s.closed() {
		panic("session is closed")
	}
	if value, exists := s.Get(key); exists {
		return value
	}

	panic("Key \"" + key + "\" does not exist")
}

// IsClosed returns the status of the connection.
func (s *Session) IsClosed() bool {
	return s.closed()
}

// GetWSConn returns the original websocket connection.
func (s *Session) GetWSConn() *ws.Conn {
	return s.conn
}
