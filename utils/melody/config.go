package melody

import "time"

/*
Melodyライブラリの設定を管理するためのConfig構造体を定義しています。
この構造体には、WebSocket接続の動作に関するさまざまな設定が含まれており、接続のタイムアウトやメッセージサイズ制限などを管理することができます。


**Config**構造体は、WebSocket接続に関するタイムアウトやメッセージのサイズ、バッファサイズなどを管理する設定です。
**newConfig**関数は、これらの設定にデフォルト値を設定し、簡単に新しいConfigインスタンスを生成できるようにしています。
この設定を使うことで、WebSocket接続の動作を細かく制御でき、パフォーマンスや安定性を調整することが可能になります。
*/

/*
WriteWait (time.Duration):

メッセージを書き込む際のタイムアウト時間です。WebSocketの書き込み操作において、接続先にメッセージを送信するまでにどれだけの時間を待機するかを設定します。指定した時間内に書き込みが完了しない場合、タイムアウトが発生して接続が終了します。
この例では「ミリ秒ではなく秒」の単位で設定されています。
PongWait (time.Duration):

クライアントからのPongメッセージ（サーバーのPingに対する応答）を待機する時間です。Pingメッセージを送信してから、クライアントからの応答をどれだけの時間待つかを設定します。指定した時間内にPongメッセージが受信されない場合、接続がタイムアウトして切断されます。
PingPeriod (time.Duration):

サーバーがクライアントにPingメッセージを送信する間隔です。WebSocket接続を維持するために定期的にPingを送り、クライアントからの応答（Pong）を確認します。この間隔は通常、PongWaitよりも短く設定されます。
この例では、PongWaitの9割（(60 * time.Second * 9) / 10）の間隔でPingメッセージが送信されるように設定されています。
MaxMessageSize (int64):

WebSocketで受信できるメッセージの最大サイズ（バイト数）を指定します。これを超えるサイズのメッセージは無効と見なされ、切断されることがあります。通常、サーバー側で大きすぎるメッセージを制限することで、リソースの過剰消費やDoS攻撃のリスクを軽減します。
MessageBufferSize (int):

各セッションでバッファに保持できるメッセージの最大数を設定します。セッションがこの数を超えるメッセージを受け取った場合、新しいメッセージを受け取る前に古いメッセージをドロップ（破棄）します。この設定は、バッファオーバーフローを防ぎ、サーバーのパフォーマンスを保つために重要です。
*/
// Config melody configuration struct.
type Config struct {
	WriteWait         time.Duration // Milliseconds until write times out.
	PongWait          time.Duration // Timeout for waiting on pong.
	PingPeriod        time.Duration // Milliseconds between pings.
	MaxMessageSize    int64         // Maximum size in bytes of a message.
	MessageBufferSize int           // The max amount of messages that can be in a sessions buffer before it starts dropping them.
}

/*
**newConfig**は、デフォルトの設定値を持つConfig構造体を生成する関数です。
WriteWaitは10秒、PongWaitは60秒、PingPeriodは約54秒（60秒の9割）に設定されています。
MaxMessageSizeは512バイト、MessageBufferSizeは256メッセージに設定されています。
これにより、WebSocket接続の動作に関する基礎的な設定が用意され、ユーザーがこれらの設定を適宜変更することも可能です。
*/
func newConfig() *Config {
	return &Config{
		WriteWait:         10 * time.Second,
		PongWait:          60 * time.Second,
		PingPeriod:        (60 * time.Second * 9) / 10,
		MaxMessageSize:    512,
		MessageBufferSize: 256,
	}
}
