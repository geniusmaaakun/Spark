package melody

import (
	"Spark/utils/cmap"
)

/*
このコードは、WebSocketサーバーにおける接続管理を効率的かつスレッドセーフに行うためのハブ機能を実装しています。ハブは以下の機能を提供します。

セッションの登録・解除: 新しいセッションを追加し、切断されたセッションを削除します。
メッセージの配信:
全セッションへのブロードキャスト
特定のセッションへの送信
条件を満たすセッションへの送信（フィルタリング）
ハブのクローズ: ハブを閉じ、すべてのセッションを安全にクローズします。
セッションの管理: セッションの数やリストを取得できます。
この実装により、WebSocketを利用したリアルタイム通信において、多数のクライアントとの接続を効率的に管理できます。また、並行マップとチャネルを活用することで、ゴルーチン間の安全な通信とデータ共有を実現しています。
*/

/*
WebSocket接続を管理するための「ハブ（hub）」と呼ばれる構造体を実装しています。
ハブは、複数のセッション（クライアントとの接続）を管理し、メッセージのブロードキャストやセッションの登録・解除などの機能を提供します。
このコードは、並行処理とチャネルを使って非同期にセッションを管理します。

このハブは、並行処理をサポートするためにチャネル（chan）とスレッドセーフなマップ（ConcurrentMap）を使用しています。
*/

/*
sessions: セッションID（UUID）とセッションへのポインタを保持する並行マップです。現在アクティブなすべてのセッションを管理します。
queue: メッセージ（envelope型）を受け取るためのチャネルです。ブロードキャストメッセージなどがここに送られます。
register: 新しいセッションを登録するためのチャネルです。新規接続時にセッションがここに送られます。
unregister: セッションを解除するためのチャネルです。切断時にセッションがここに送られます。
exit: ハブを終了させるためのチャネルです。クローズメッセージがここに送られます。
open: ハブが開いている（新しいセッションを受け付ける）かどうかを示すブール値です。
*/
type hub struct {
	sessions   cmap.ConcurrentMap[string, *Session]
	queue      chan *envelope
	register   chan *Session
	unregister chan *Session
	exit       chan *envelope
	open       bool
}

/*
newHub: 新しいhubインスタンスを作成します。この関数はhubを初期化し、各種チャネルを準備します。
sessionsには、セッションを格納するスレッドセーフなマップが設定されます。
他のチャネル（queue、register、unregister、exit）も、非同期処理のために初期化されます。
openは、ハブが動作中かどうかを示すフラグで、初期値はtrueです。
*/
func newHub() *hub {
	return &hub{
		sessions:   cmap.New[*Session](),
		queue:      make(chan *envelope),
		register:   make(chan *Session),
		unregister: make(chan *Session),
		exit:       make(chan *envelope),
		open:       true,
	}
}

//runメソッド: ハブのメインループであり、ゴルーチンとして実行されます。このループでは、チャネルを介して送られてくるさまざまなイベントを処理します。
func (h *hub) run() {
	/*
		h.registerからの受信:
		新しいセッションを登録します。
		h.openがtrueの場合のみセッションを追加します。
		h.unregisterからの受信:
		セッションを解除（削除）します。
		h.queueからの受信:
		メッセージをセッションに配信します。メッセージの配信方法は以下で詳しく説明します。
		h.exitからの受信:
		ハブを閉じ、すべてのセッションをクローズします。
		ループを抜けて処理を終了します。
	*/
loop:
	for {
		select {
		case s := <-h.register:
			if h.open {
				h.sessions.Set(s.UUID, s)
			}
		case s := <-h.unregister:
			h.sessions.Remove(s.UUID)

			//メッセージの配信処理
			/*
				m := <-h.queue: キューからメッセージを受信します。このメッセージはenvelope型で、送信する内容や対象セッションの情報を持っています。
				m.listが存在する場合:
				m.listは特定のセッションUUIDのリストです。指定されたセッションにのみメッセージを送信します。
				ループで各UUIDについてセッションを取得し、メッセージを送信します。
				m.filterがnilの場合:
				すべてのセッションにメッセージを送信します。
				h.sessions.IterCbを使って、全セッションに対してwriteMessageを呼び出します。
				m.filterが存在する場合:
				フィルタ関数m.filterを使って、条件を満たすセッションにのみメッセージを送信します。
				各セッションについてフィルタを適用し、trueの場合にメッセージを送信します。
			*/
		case m := <-h.queue:
			if len(m.list) > 0 {
				for _, uuid := range m.list {
					if s, ok := h.sessions.Get(uuid); ok {
						s.writeMessage(m)
					}
				}
			} else if m.filter == nil {
				h.sessions.IterCb(func(uuid string, s *Session) bool {
					s.writeMessage(m)
					return true
				})
			} else {
				h.sessions.IterCb(func(uuid string, s *Session) bool {
					if m.filter(s) {
						s.writeMessage(m)
					}
					return true
				})
			}
			//ハブのクローズ処理
			/*
				m := <-h.exit: exitチャネルからクローズメッセージを受信します。
				h.open = false: ハブを閉じ、新しいセッションの登録を停止します。
				全セッションのクローズ:
				h.sessions.IterCbで全セッションを巡回します。
				各セッションに対してクローズメッセージを送信し、セッションをクローズします。
				セッションのUUIDをkeysに収集します。
				セッションの削除:
				収集したUUIDを使って、セッションをh.sessionsから削除します。
				break loop: ループを抜けてrunメソッドを終了します。
			*/
		case m := <-h.exit:
			var keys []string
			h.open = false
			h.sessions.IterCb(func(uuid string, s *Session) bool {
				s.writeMessage(m)
				s.Close()
				keys = append(keys, uuid)
				return true
			})
			for i := range keys {
				h.sessions.Remove(keys[i])
			}
			break loop
		}
	}
}

// closed: ハブが閉じているかどうかを返します。
// len: 現在アクティブなセッションの数を返します。
// list: 現在アクティブなセッションのUUIDのリストを返します。

func (h *hub) closed() bool {
	return !h.open
}

func (h *hub) len() int {
	return h.sessions.Count()
}

func (h *hub) list() []string {
	return h.sessions.Keys()
}
