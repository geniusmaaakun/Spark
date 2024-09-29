package desktop

import (
	"Spark/modules"
	"Spark/server/common"
	"Spark/server/handler/utility"
	"Spark/utils"
	"Spark/utils/melody"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

/*
デスクトップリモートセッションを管理するためのWebSocketベースのサーバー機能を提供しています。
GinとMelodyライブラリを使って、ブラウザやリモートデバイス間でのデスクトップリモートセッションの作成、メッセージのやり取り、セッションの終了などを管理します。


デスクトップのリモートセッションを管理するためのWebSocketベースのサーバー機能です。デスクトップセッションは、クライアント（ブラウザ）とリモートデバイス間でのやり取りを管理します。データの送受信、セッションの管理、セッション終了時のクリーンアップが行われます。
*/

/*
desktop構造体: デスクトップセッションを管理するための構造体です。リモートデスクトップセッションのUUID、関連するデバイスのID、ブラウザセッション(srcConn)、デバイスセッション(deviceConn)を保持します。

desktopSessions: Melodyを使ってWebSocketセッションを管理するオブジェクトです。クライアントやデバイス間の通信を管理し、接続やメッセージ送信時のイベントハンドリングを行います。
*/
type desktop struct {
	uuid       string
	device     string
	srcConn    *melody.Session
	deviceConn *melody.Session
}

var desktopSessions = melody.New()

func init() {
	desktopSessions.Config.MaxMessageSize = common.MaxMessageSize
	desktopSessions.HandleConnect(onDesktopConnect)
	desktopSessions.HandleMessage(onDesktopMessage)
	desktopSessions.HandleMessageBinary(onDesktopMessage)
	desktopSessions.HandleDisconnect(onDesktopDisconnect)
	go utility.WSHealthCheck(desktopSessions, sendPack)
}

/*
InitDesktop: クライアントがWebSocket接続を開始するためのエンドポイント。クエリパラメータとしてsecretとdeviceを受け取り、WebSocketハンドシェイクを行います。
WebSocketでないリクエストは400 Bad Requestを返して拒否します。
クエリパラメータsecretの長さが32バイトでなければエラーを返します。
deviceが有効なデバイスIDでなければセッションを開始せずに終了します。
*/
// InitDesktop handles desktop websocket handshake event
func InitDesktop(ctx *gin.Context) {
	if !ctx.IsWebsocket() {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	secretStr, ok := ctx.GetQuery(`secret`)
	if !ok || len(secretStr) != 32 {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	secret, err := hex.DecodeString(secretStr)
	if err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	device, ok := ctx.GetQuery(`device`)
	if !ok {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if _, ok := common.CheckDevice(device, ``); !ok {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	desktopSessions.HandleRequestWithKeys(ctx.Writer, ctx.Request, gin.H{
		`Secret`:   secret,
		`Device`:   device,
		`LastPack`: utils.Unix,
	})
}

/*
desktopEventWrapper: デバイスからブラウザに対して送信されるパケットの処理を行う関数をラップするためのコールバック関数です。
イベントRAW_DATA_ARRIVEなど、デバイスから送られてきた生データに応じて、データをブラウザに送信するかどうかを決定します。
DESKTOP_INIT: セッション初期化が成功したか失敗したかを確認し、失敗した場合はエラーメッセージをブラウザに送信します。
DESKTOP_QUIT: セッションが終了した際に、ブラウザに終了メッセージを送信します。
*/
// desktopEventWrapper returns a eventCallback function that will
// be called when device need to send a packet to browser
func desktopEventWrapper(desktop *desktop) common.EventCallback {
	return func(pack modules.Packet, device *melody.Session) {
		if pack.Act == `RAW_DATA_ARRIVE` && pack.Data != nil {
			data := *pack.Data[`data`].(*[]byte)
			if data[5] == 00 || data[5] == 01 || data[5] == 02 {
				desktop.srcConn.WriteBinary(data)
				return
			}

			if data[5] != 03 {
				return
			}
			data = data[8:]
			data = utility.SimpleDecrypt(data, device)
			if utils.JSON.Unmarshal(data, &pack) != nil {
				return
			}
		}

		switch pack.Act {
		case `DESKTOP_INIT`:
			if pack.Code != 0 {
				msg := `${i18n|DESKTOP.CREATE_SESSION_FAILED}`
				if len(pack.Msg) > 0 {
					msg += `: ` + pack.Msg
				} else {
					msg += `${i18n|COMMON.UNKNOWN_ERROR}`
				}
				sendPack(modules.Packet{Act: `QUIT`, Msg: msg}, desktop.srcConn)
				common.RemoveEvent(desktop.uuid)
				desktop.srcConn.Close()
				common.Warn(desktop.srcConn, `DESKTOP_INIT`, `fail`, msg, map[string]any{
					`deviceConn`: desktop.deviceConn,
				})
			} else {
				common.Info(desktop.srcConn, `DESKTOP_INIT`, `success`, ``, map[string]any{
					`deviceConn`: desktop.deviceConn,
				})
			}
		case `DESKTOP_QUIT`:
			msg := `${i18n|DESKTOP.SESSION_CLOSED}`
			if len(pack.Msg) > 0 {
				msg = pack.Msg
			}
			sendPack(modules.Packet{Act: `QUIT`, Msg: msg}, desktop.srcConn)
			common.RemoveEvent(desktop.uuid)
			desktop.srcConn.Close()
			common.Info(desktop.srcConn, `DESKTOP_QUIT`, `success`, ``, map[string]any{
				`deviceConn`: desktop.deviceConn,
			})
		}
	}
}

/*
**onDesktopConnect**は、新しいデスクトップセッションが接続された際に呼ばれるハンドラーです。
WebSocket接続時にDeviceが取得できなければ、セッションを閉じてエラーメッセージを送信します。
desktopインスタンスを作成し、セッションに関連付けます。これにより、デスクトップセッションが初期化され、通信が可能になります。
*/
func onDesktopConnect(session *melody.Session) {
	device, ok := session.Get(`Device`)
	if !ok {
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|DESKTOP.CREATE_SESSION_FAILED}`}, session)
		session.Close()
		return
	}
	connUUID, ok := common.CheckDevice(device.(string), ``)
	if !ok {
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|COMMON.DEVICE_NOT_EXIST}`}, session)
		session.Close()
		return
	}
	deviceConn, ok := common.Melody.GetSessionByUUID(connUUID)
	if !ok {
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|COMMON.DEVICE_NOT_EXIST}`}, session)
		session.Close()
		return
	}
	desktopUUID := utils.GetStrUUID()
	desktop := &desktop{
		uuid:       desktopUUID,
		device:     device.(string),
		srcConn:    session,
		deviceConn: deviceConn,
	}
	session.Set(`Desktop`, desktop)
	common.AddEvent(desktopEventWrapper(desktop), connUUID, desktopUUID)
	common.SendPack(modules.Packet{Act: `DESKTOP_INIT`, Data: gin.H{
		`desktop`: desktopUUID,
	}, Event: desktopUUID}, deviceConn)
	common.Info(desktop.srcConn, `DESKTOP_CONN`, `success`, ``, map[string]any{
		`deviceConn`: desktop.deviceConn,
	})
}

/*
**onDesktopMessage**は、デスクトップセッションからのメッセージを処理します。
バイナリパケットの検証: パケットが有効であるかを確認します。無効な場合はセッションを閉じてエラーを返します。
メッセージ内容に応じた処理: 受け取ったパケットのActに応じて、DESKTOP_PINGやDESKTOP_KILL、DESKTOP_SHOTなどの操作を行います。
*/
func onDesktopMessage(session *melody.Session, data []byte) {
	var pack modules.Packet
	val, ok := session.Get(`Desktop`)
	if !ok {
		return
	}
	desktop := val.(*desktop)

	service, op, isBinary := utils.CheckBinaryPack(data)
	if !isBinary || service != 20 {
		sendPack(modules.Packet{Code: -1}, session)
		session.Close()
		return
	}
	if op != 03 {
		sendPack(modules.Packet{Code: -1}, session)
		session.Close()
		return
	}

	data = utility.SimpleDecrypt(data[8:], session)
	if utils.JSON.Unmarshal(data, &pack) != nil {
		sendPack(modules.Packet{Code: -1}, session)
		session.Close()
		return
	}
	session.Set(`LastPack`, utils.Unix)

	switch pack.Act {
	case `DESKTOP_PING`:
		common.SendPack(modules.Packet{Act: `DESKTOP_PING`, Data: gin.H{
			`desktop`: desktop.uuid,
		}, Event: desktop.uuid}, desktop.deviceConn)
		return
	case `DESKTOP_KILL`:
		common.Info(desktop.srcConn, `DESKTOP_KILL`, `success`, ``, map[string]any{
			`deviceConn`: desktop.deviceConn,
		})
		common.SendPack(modules.Packet{Act: `DESKTOP_KILL`, Data: gin.H{
			`desktop`: desktop.uuid,
		}, Event: desktop.uuid}, desktop.deviceConn)
		return
	case `DESKTOP_SHOT`:
		common.SendPack(modules.Packet{Act: `DESKTOP_SHOT`, Data: gin.H{
			`desktop`: desktop.uuid,
		}, Event: desktop.uuid}, desktop.deviceConn)
		return
	}
	session.Close()
}

/*
**onDesktopDisconnect**は、デスクトップセッションが切断された際に呼ばれます。
セッション切断時に、デバイスにセッションが終了したことを通知し、イベントやセッション情報をクリアします。
*/
func onDesktopDisconnect(session *melody.Session) {
	common.Info(session, `DESKTOP_CLOSE`, `success`, ``, nil)
	val, ok := session.Get(`Desktop`)
	if !ok {
		return
	}
	desktop, ok := val.(*desktop)
	if !ok {
		return
	}
	common.SendPack(modules.Packet{Act: `DESKTOP_KILL`, Data: gin.H{
		`desktop`: desktop.uuid,
	}, Event: desktop.uuid}, desktop.deviceConn)
	common.RemoveEvent(desktop.uuid)
	session.Set(`Desktop`, nil)
	desktop = nil
}

//sendPack: 任意のパケットをWebSocketセッションに送信する関数で、データをシリアライズして暗号化し、バイナリデータとして送信します。
/*
機能説明:
目的: この関数は、modules.Packet（パケット）を特定のWebSocketセッションにバイナリデータとして送信します。
引数:
pack: 送信するデータ（パケット）。JSON形式で送信するため、まずシリアライズ（マーシャリング）されます。
session: データを送信する対象のWebSocketセッション（*melody.Session型）。
処理の流れ:
セッションの存在確認: セッションがnilでないか確認します。nilの場合はfalseを返して終了します。
パケットのシリアライズ: パケットをutils.JSON.Marshalを使ってJSON形式に変換します。シリアライズに失敗した場合もfalseを返します。
データの暗号化: utility.SimpleEncryptを使って、シリアライズされたデータを暗号化します。この暗号化は、セッション情報に基づいて行われます。
データの送信:
暗号化されたデータに特定のバイト列（34, 22, 19, 17, 20, 03）を先頭に付与し、session.WriteBinaryを使ってセッションにバイナリデータとして送信します。
エラーが発生した場合はfalse、成功した場合はtrueを返します。
特徴:
バイナリデータ送信時に、先頭に特定の6バイトのプレフィックス（34, 22, 19, 17, 20, 03）を付与しています。これはおそらく通信のプロトコルを識別するためのものです。
暗号化されたデータを送信するため、通信の安全性が考慮されています。
*/
func sendPack(pack modules.Packet, session *melody.Session) bool {
	if session == nil {
		return false
	}
	data, err := utils.JSON.Marshal(pack)
	if err != nil {
		return false
	}
	data = utility.SimpleEncrypt(data, session)
	err = session.WriteBinary(append([]byte{34, 22, 19, 17, 20, 03}, data...))
	return err == nil
}

//CloseSessionsByDevice: 特定のデバイスIDに関連するすべてのWebSocketセッションを終了させるための関数で、各セッションに終了通知を送信してからセッションを閉じます。
/*
機能説明:
目的: 特定のdeviceIDに関連するすべてのデスクトップセッションを閉じるための関数です。
引数:
deviceID: 対象とするデバイスのID。
処理の流れ:
セッションのイテレーション:

desktopSessions.IterSessionsを使って、すべてのデスクトップセッションを繰り返し処理します。
それぞれのセッションについて、session.Get("Desktop")でDesktopというキーに関連する値を取得し、その値が存在し、かつ正しい型（*desktop型）であるか確認します。
デバイスIDの一致確認:

desktop.deviceが引数として渡されたdeviceIDと一致するか確認します。
一致する場合、そのセッションにQUITメッセージ（セッション終了の通知）を送信し、セッションを閉じるためのリストに追加します。
セッションのクローズ:

すべての対象セッションに対して、session.Close()を呼び出し、セッションを閉じます。
特徴:
デバイスに関連するセッションを安全にクローズします。セッションをただ閉じるだけでなく、まずそのセッションに終了通知（QUIT）を送信してから閉じます。
クライアントに対して終了通知を送ることで、ユーザにセッションの終了を知らせることができます。
*/
func CloseSessionsByDevice(deviceID string) {
	var queue []*melody.Session
	desktopSessions.IterSessions(func(_ string, session *melody.Session) bool {
		val, ok := session.Get(`Desktop`)
		if !ok {
			return true
		}
		desktop, ok := val.(*desktop)
		if !ok {
			return true
		}
		if desktop.device == deviceID {
			sendPack(modules.Packet{Act: `QUIT`, Msg: `${i18n|DESKTOP.SESSION_CLOSED}`}, desktop.srcConn)
			queue = append(queue, session)
			return false
		}
		return true
	})
	for _, session := range queue {
		session.Close()
	}
}
