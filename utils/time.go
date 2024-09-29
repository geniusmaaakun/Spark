package utils

import "time"

//Now: 現在の時刻を保持するための変数。time.Time型です。
//Unix: 現在のUnixタイムスタンプ（1970年1月1日からの秒数）を保持するための変数。int64型です。
var Now time.Time = time.Now()
var Unix int64 = Now.Unix()

// To prevent call time.Now().Unix() too often.
/*
**init()**関数は、Goプログラムのパッケージが初期化される際に自動的に実行される特別な関数です。このコードでは、init関数の中でゴルーチン（軽量スレッド）を開始しています。

time.NewTicker(time.Second): 毎秒イベントを生成するTickerを作成しています。これは1秒ごとにチャネルCに現在時刻を送信します。

time.NewTicker(time.Second).Cから1秒ごとに受信する現在時刻nowを、グローバル変数NowとUnixにそれぞれ更新しています。
これにより、Nowは常に最新の現在時刻を保持し、Unixは最新のUnixタイムスタンプを保持します。



通常、time.Now()やtime.Now().Unix()を頻繁に呼び出すと、その都度システムから現在時刻を取得する処理が行われます。このコードは、1秒に1回だけ現在時刻を更新するようにして、不要なシステムコールを減らし、効率的に時刻情報を提供するようになっています。

これにより、他のコードがNowやUnixを参照すると、最新の時刻を即座に取得できますが、過度にtime.Now()やtime.Now().Unix()を呼び出す必要がありません。
*/
func init() {
	go func() {
		for now := range time.NewTicker(time.Second).C {
			Now = now
			Unix = now.Unix()
		}
	}()
}
