package common

import (
	"Spark/client/config"
	"Spark/modules"
	"Spark/utils"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/imroc/req/v3"
)

/*
WebSocket 接続を管理し、データ送信や暗号化された通信を行うための Go 言語での実装です。
Conn 構造体を中心に、WebSocket 接続や HTTP 通信を扱うメソッドが定義されています。以下に、各部分を詳しく説明します。
*/

/*
Conn: *ws.Conn 型を埋め込んでおり、Gorilla WebSocket ライブラリの Conn 構造体に加え、secret と secretHex を追加しています。
secret は通信に使われるバイト配列で、secretHex はその16進数表現です。
*/
type Conn struct {
	*ws.Conn
	secret    []byte
	secretHex string
}

//MaxMessageSize: WebSocket 経由で送信可能な最大メッセージサイズを定義しています。ここでは約 66 KB (2^15 + 1024 バイト) です。
const MaxMessageSize = (2 << 15) + 1024

/*
WSConn: WebSocket 接続を管理するためのグローバル変数です。*Conn 型で、現在の WebSocket 接続を保持します。
Mutex: 同時に複数のゴルーチンが WebSocket にアクセスしないようにするためのミューテックスです。
HTTP: req ライブラリを使って HTTP クライアントを作成します。CreateClient() 関数で生成され、HTTP リクエストを送信する際に使用されます。
*/
var WSConn *Conn
var Mutex = &sync.Mutex{}
var HTTP = CreateClient()

//CreateConn: WebSocket 接続 ws.Conn と暗号化用の secret を受け取り、それを基に Conn 構造体を作成して返す関数です。
func CreateConn(wsConn *ws.Conn, secret []byte) *Conn {
	return &Conn{
		Conn:      wsConn,
		secret:    secret,
		secretHex: hex.EncodeToString(secret),
	}
}

//CreateClient: req ライブラリを使って HTTP クライアントを生成します。ここでは、クライアントの User-Agent を設定しています。
func CreateClient() *req.Client {
	return req.C().SetUserAgent(`SPARK COMMIT: ` + config.COMMIT)
}

//SendData: WebSocket 経由でバイナリデータを送信する関数です。Mutex を使って排他制御を行い、データが正常に送信されるようにします。データは ws.BinaryMessage 形式で送信されます。
func (wsConn *Conn) SendData(data []byte) error {
	Mutex.Lock()
	defer Mutex.Unlock()
	if WSConn == nil {
		return errors.New(`${i18n|COMMON.DISCONNECTED}`)
	}
	wsConn.SetWriteDeadline(utils.Now.Add(5 * time.Second))
	defer wsConn.SetWriteDeadline(time.Time{})
	return wsConn.WriteMessage(ws.BinaryMessage, data)
}

//SendPack: 送信するパケット pack を JSON に変換し、暗号化してから送信します。データが大きすぎる場合は、HTTP 経由で送信し、そうでなければ WebSocket 経由で送信します。
func (wsConn *Conn) SendPack(pack any) error {
	Mutex.Lock()
	defer Mutex.Unlock()
	data, err := utils.JSON.Marshal(pack)
	if err != nil {
		return err
	}
	data, err = utils.Encrypt(data, wsConn.secret)
	if err != nil {
		return err
	}
	if len(data) > MaxMessageSize {
		_, err = HTTP.R().
			SetBody(data).
			SetHeader(`Secret`, wsConn.secretHex).
			Send(`POST`, config.GetBaseURL(false)+`/ws`)
		return err
	}
	if WSConn == nil {
		return errors.New(`${i18n|COMMON.DISCONNECTED}`)
	}
	wsConn.SetWriteDeadline(utils.Now.Add(5 * time.Second))
	defer wsConn.SetWriteDeadline(time.Time{})
	return wsConn.WriteMessage(ws.BinaryMessage, data)
}

//SendRawData: Raw データ（バイナリデータ）を送信する関数です。event、service、op を含むヘッダーを設定してからデータを送信します。
func (wsConn *Conn) SendRawData(event, data []byte, service byte, op byte) error {
	Mutex.Lock()
	defer Mutex.Unlock()
	if WSConn == nil {
		return errors.New(`${i18n|COMMON.DISCONNECTED}`)
	}
	buffer := make([]byte, 24)
	copy(buffer[6:22], event)
	copy(buffer[:4], []byte{34, 22, 19, 17})
	buffer[4] = service
	buffer[5] = op
	binary.BigEndian.PutUint16(buffer[22:24], uint16(len(data)))
	buffer = append(buffer, data...)

	wsConn.SetWriteDeadline(utils.Now.Add(5 * time.Second))
	defer wsConn.SetWriteDeadline(time.Time{})
	return wsConn.WriteMessage(ws.BinaryMessage, buffer)
}

//SendCallback: 送信するパケット pack に前回のイベント情報 prev を含めて送信します。
func (wsConn *Conn) SendCallback(pack, prev modules.Packet) error {
	if len(prev.Event) > 0 {
		pack.Event = prev.Event
	}
	return wsConn.SendPack(pack)
}

//GetSecret, GetSecretHex: Conn 構造体に保存されている secret をそのまま取得するためのゲッターです。
func (wsConn *Conn) GetSecret() []byte {
	return wsConn.secret
}

func (wsConn *Conn) GetSecretHex() string {
	return wsConn.secretHex
}
