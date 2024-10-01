package terminal

import (
	"Spark/modules"
	"Spark/server/common"
	"Spark/server/handler/utility"
	"Spark/utils"
	"Spark/utils/melody"
	"encoding/hex"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
)

/*
リモートデバイス上でターミナルセッションを管理するためのAPIを実装しています。
リモートデバイスとブラウザ間でのターミナル入力・出力をWebSocketを使って処理しています。以下にコードの各部分の詳細な解説を行います。

リモートデバイスとブラウザ間のターミナルセッションをWebSocketを使って実装しています。特定のデバイスに対してターミナルを開き、コマンドの送受信やセッションの管理を行うための仕組みが備わっています。
*/

/*
uuid: ターミナルセッションの一意なID。
device: 接続されているリモートデバイスのID。
session: ブラウザとのWebSocketセッション。
deviceConn: リモートデバイスとのWebSocketセッション。
*/
type terminal struct {
	uuid       string
	device     string
	session    *melody.Session
	deviceConn *melody.Session
}

//terminalSessions は、リモートデバイスとブラウザ間のWebSocketセッションを管理するための melody ライブラリを使用しています。
var terminalSessions = melody.New()

/*
MaxMessageSize: WebSocketで送信できるメッセージの最大サイズを設定。
HandleConnect: 新しいWebSocket接続が確立されたときに onTerminalConnect が呼び出されます。
HandleMessage: テキストまたはバイナリメッセージが受信されたときに onTerminalMessage が呼び出されます。
HandleDisconnect: WebSocket接続が切断されたときに onTerminalDisconnect が呼び出されます。
WSHealthCheck: WebSocketのヘルスチェックを行い、アクティブでない接続をクリーンアップする機能。
*/
func init() {
	terminalSessions.Config.MaxMessageSize = common.MaxMessageSize
	terminalSessions.HandleConnect(onTerminalConnect)
	terminalSessions.HandleMessage(onTerminalMessage)
	terminalSessions.HandleMessageBinary(onTerminalMessage)
	terminalSessions.HandleDisconnect(onTerminalDisconnect)
	go utility.WSHealthCheck(terminalSessions, sendPack)
}

/*
WebSocketの初期化処理です。secretとdeviceというパラメータをクエリから取得し、それを検証します。
クライアントがWebSocketで接続していることを確認し、terminalSessionsにセッションを登録します。
*/
// InitTerminal handles terminal websocket handshake event
func InitTerminal(ctx *gin.Context) {
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

	terminalSessions.HandleRequestWithKeys(ctx.Writer, ctx.Request, gin.H{
		`Secret`:   secret,
		`Device`:   device,
		`LastPack`: utils.Unix,
	})
}

/*
この関数はターミナルイベントのラッパーです。リモートデバイスからターミナルにデータが送信された場合、そのデータを処理してブラウザに返します。
TERMINAL_INIT や TERMINAL_OUTPUT などのイベントに応じて処理を分岐させます。
*/
// terminalEventWrapper returns a eventCallback function that will
// be called when device need to send a packet to browser
func terminalEventWrapper(terminal *terminal) common.EventCallback {
	return func(pack modules.Packet, device *melody.Session) {
		if pack.Act == `RAW_DATA_ARRIVE` && pack.Data != nil {
			data := *pack.Data[`data`].(*[]byte)
			if data[5] == 00 {
				terminal.session.WriteBinary(data)
				return
			}

			if data[5] != 01 {
				return
			}
			data = data[8:]
			data = utility.SimpleDecrypt(data, device)
			if utils.JSON.Unmarshal(data, &pack) != nil {
				return
			}
		}

		switch pack.Act {
		case `TERMINAL_INIT`:
			if pack.Code != 0 {
				msg := `${i18n|TERMINAL.CREATE_SESSION_FAILED}`
				if len(pack.Msg) > 0 {
					msg += `: ` + pack.Msg
				} else {
					msg += `${i18n|COMMON.UNKNOWN_ERROR}`
				}
				sendPack(modules.Packet{Act: `QUIT`, Msg: msg}, terminal.session)
				common.RemoveEvent(terminal.uuid)
				terminal.session.Close()
				common.Warn(terminal.session, `TERMINAL_INIT`, `fail`, msg, map[string]any{
					`deviceConn`: terminal.deviceConn,
				})
			} else {
				common.Info(terminal.session, `TERMINAL_INIT`, `success`, ``, map[string]any{
					`deviceConn`: terminal.deviceConn,
				})
			}
		case `TERMINAL_QUIT`:
			msg := `${i18n|TERMINAL.SESSION_CLOSED}`
			if len(pack.Msg) > 0 {
				msg = pack.Msg
			}
			sendPack(modules.Packet{Act: `QUIT`, Msg: msg}, terminal.session)
			common.RemoveEvent(terminal.uuid)
			terminal.session.Close()
			common.Info(terminal.session, `TERMINAL_QUIT`, ``, msg, map[string]any{
				`deviceConn`: terminal.deviceConn,
			})
		case `TERMINAL_OUTPUT`:
			if pack.Data == nil {
				return
			}
			if output, ok := pack.Data[`output`]; ok {
				sendPack(modules.Packet{Act: `TERMINAL_OUTPUT`, Data: gin.H{
					`output`: output,
				}}, terminal.session)
			}
		}
	}
}

/*
WebSocket接続が確立された際に呼び出されるコールバック関数です。
デバイスの存在を確認し、セッションを初期化します。
*/
func onTerminalConnect(session *melody.Session) {
	device, ok := session.Get(`Device`)
	if !ok {
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|TERMINAL.CREATE_SESSION_FAILED}`}, session)
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
	uuid := utils.GetStrUUID()
	terminal := &terminal{
		uuid:       uuid,
		device:     device.(string),
		session:    session,
		deviceConn: deviceConn,
	}
	session.Set(`Terminal`, terminal)
	common.AddEvent(terminalEventWrapper(terminal), connUUID, uuid)
	common.SendPack(modules.Packet{Act: `TERMINAL_INIT`, Data: gin.H{
		`terminal`: uuid,
	}, Event: uuid}, deviceConn)
	common.Info(terminal.session, `TERMINAL_CONN`, `success`, ``, map[string]any{
		`deviceConn`: terminal.deviceConn,
	})
}

/*
WebSocket経由で受信したメッセージを処理します。
バイナリメッセージかどうかを確認し、適切に処理を振り分けます。
*/
func onTerminalMessage(session *melody.Session, data []byte) {
	var pack modules.Packet
	val, ok := session.Get(`Terminal`)
	if !ok {
		return
	}
	terminal := val.(*terminal)

	service, op, isBinary := utils.CheckBinaryPack(data)
	if !isBinary || service != 21 {
		sendPack(modules.Packet{Code: -1}, session)
		session.Close()
		return
	}
	if op == 00 {
		session.Set(`LastPack`, utils.Unix)
		rawEvent, _ := hex.DecodeString(terminal.uuid)
		data = append(data, rawEvent...)
		copy(data[22:], data[6:])
		copy(data[6:], rawEvent)
		terminal.deviceConn.WriteBinary(data)
		return
	}
	if op != 01 {
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
	case `TERMINAL_INPUT`:
		if pack.Data == nil {
			return
		}
		if input, ok := pack.GetData(`input`, reflect.String); ok {
			rawInput, _ := hex.DecodeString(input.(string))
			common.Info(terminal.session, `TERMINAL_INPUT`, ``, ``, map[string]any{
				`deviceConn`: terminal.deviceConn,
				`input`:      utils.BytesToString(rawInput),
			})
			common.SendPack(modules.Packet{Act: `TERMINAL_INPUT`, Data: gin.H{
				`input`:    input,
				`terminal`: terminal.uuid,
			}, Event: terminal.uuid}, terminal.deviceConn)
		}
		return
	case `TERMINAL_RESIZE`:
		if pack.Data == nil {
			return
		}
		if cols, ok := pack.Data[`cols`]; ok {
			if rows, ok := pack.Data[`rows`]; ok {
				common.SendPack(modules.Packet{Act: `TERMINAL_RESIZE`, Data: gin.H{
					`cols`:     cols,
					`rows`:     rows,
					`terminal`: terminal.uuid,
				}, Event: terminal.uuid}, terminal.deviceConn)
			}
		}
		return
	case `TERMINAL_KILL`:
		common.Info(terminal.session, `TERMINAL_KILL`, `success`, ``, map[string]any{
			`deviceConn`: terminal.deviceConn,
		})
		common.SendPack(modules.Packet{Act: `TERMINAL_KILL`, Data: gin.H{
			`terminal`: terminal.uuid,
		}, Event: terminal.uuid}, terminal.deviceConn)
		return
	case `PING`:
		common.SendPack(modules.Packet{Act: `TERMINAL_PING`, Data: gin.H{
			`terminal`: terminal.uuid,
		}, Event: terminal.uuid}, terminal.deviceConn)
		return
	}
	session.Close()
}

/*
WebSocketが切断された際に呼び出されます。
セッションのクリーンアップを行い、関連するリソースを解放します。
*/
func onTerminalDisconnect(session *melody.Session) {
	common.Info(session, `TERMINAL_CLOSE`, `success`, ``, nil)
	val, ok := session.Get(`Terminal`)
	if !ok {
		return
	}
	terminal, ok := val.(*terminal)
	if !ok {
		return
	}
	common.SendPack(modules.Packet{Act: `TERMINAL_KILL`, Data: gin.H{
		`terminal`: terminal.uuid,
	}, Event: terminal.uuid}, terminal.deviceConn)
	common.RemoveEvent(terminal.uuid)
	session.Set(`Terminal`, nil)
	terminal = nil
}

//ターミナルセッションにデータを送信するための関数です。
func sendPack(pack modules.Packet, session *melody.Session) bool {
	if session == nil {
		return false
	}
	data, err := utils.JSON.Marshal(pack)
	if err != nil {
		return false
	}
	data = utility.SimpleEncrypt(data, session)
	err = session.WriteBinary(data)
	return err == nil
}

//指定されたデバイスIDに関連するすべてのターミナルセッションを閉じます。
func CloseSessionsByDevice(deviceID string) {
	var queue []*melody.Session
	terminalSessions.IterSessions(func(_ string, session *melody.Session) bool {
		val, ok := session.Get(`Terminal`)
		if !ok {
			return true
		}
		terminal, ok := val.(*terminal)
		if !ok {
			return true
		}
		if terminal.device == deviceID {
			queue = append(queue, session)
			return false
		}
		return true
	})
	for _, session := range queue {
		session.Close()
	}
}
