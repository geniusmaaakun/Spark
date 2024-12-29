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

// sessionsの設定
// ハンドラーの設定
// ヘルスチェック
func init() {
	desktopSessions.Config.MaxMessageSize = common.MaxMessageSize
	// 各ハンドラをセット
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
// デスクトップセッションを初期化するための処理を行います。具体的には、クライアントからのWebSocketリクエストを受け取り、セッションを確立します。
func InitDesktop(ctx *gin.Context) {
	//リクエストがWebSocketであることを確認
	if !ctx.IsWebsocket() {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	//secret クエリパラメータの取得と検証
	//クライアントとサーバー間のセッションを識別するための32文字の16進文字列。
	secretStr, ok := ctx.GetQuery(`secret`)
	//存在しない、または32文字でない場合、処理を終了。
	if !ok || len(secretStr) != 32 {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	//hex.DecodeString を使って16進文字列をバイト配列に変換。
	secret, err := hex.DecodeString(secretStr)
	if err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	//device パラメータの取得と検証
	//device の役割 セッションに関連付けられるデバイスの一意な識別子。
	device, ok := ctx.GetQuery(`device`)
	if !ok {
		//存在しない場合、400 Bad Request を返して終了
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	//common.CheckDevice を使用して、デバイスが有効で登録されているか確認。
	if _, ok := common.CheckDevice(device, ``); !ok {
		//無効な場合、エラーを返す。
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	//セッションの初期化
	//desktopSessions にリクエストを登録し、セッションを初期化します。
	// Secret: セッションの識別用に使用される秘密鍵。
	// Device: デスクトップセッションに関連付けられたデバイス。
	// LastPack: セッションの最後のリクエスト時間（Unixタイムスタンプ）。
	//WebSocketリクエストを受け取り、セッション管理用のデータ構造に追加。
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
//リモートデスクトップセッションのイベントを処理するためのイベントハンドラーを提供します。
//関数 desktopEventWrapper は、desktop というセッション情報を引数に取り、common.EventCallback 型のコールバック関数を生成して返します。
//このコールバックは、リモートデスクトップに関連するイベントを受信し、それに応じた処理を実行します。
func desktopEventWrapper(desktop *desktop) common.EventCallback {
	return func(pack modules.Packet, device *melody.Session) {
		//pack.Act == "RAW_DATA_ARRIVE" の場合に、イベントデータ（pack.Data）が処理されます。
		if pack.Act == `RAW_DATA_ARRIVE` && pack.Data != nil {
			data := *pack.Data[`data`].(*[]byte)
			//値が 00, 01, 02 の場合:
			// データをそのまま desktop.srcConn.WriteBinary(data) に送信。
			// これにより、リモートデスクトップのクライアントにそのままバイナリデータが転送されます。
			// 処理を終了（return）
			if data[5] == 00 || data[5] == 01 || data[5] == 02 {
				desktop.srcConn.WriteBinary(data)
				return
			}

			if data[5] != 03 {
				return
			}

			//値 03: データを復号化して処理。
			// 値が 03 の場合:
			// データの8バイト目以降を抽出。
			// utility.SimpleDecrypt を使用してデータをデバイスセッションに基づいて復号化。
			// 復号化したデータを modules.Packet にデシリアライズ。
			// デシリアライズが成功しなければ処理を終了。
			data = data[8:]
			data = utility.SimpleDecrypt(data, device)
			if utils.JSON.Unmarshal(data, &pack) != nil {
				return
			}
		}

		switch pack.Act {
		//DESKTOP_INIT (セッション初期化)
		case `DESKTOP_INIT`:
			// pack.Code が 0 以外（エラーが発生）かどうかを判定します。
			// エラーの場合:
			// エラーメッセージを構築。
			// sendPack を使ってエラーをクライアントに送信。
			// イベントリスナーを削除し、リソースをクリーンアップ（common.RemoveEvent や desktop.srcConn.Close）。
			// エラー情報をログに記録（common.Warn）。
			// 成功の場合:
			// ログに成功情報を記録（common.Info）。
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
			//DESKTOP_QUIT (セッション終了)
			// セッションが終了したことを示すメッセージをクライアントに送信。
			// イベントリスナーを削除し、リソースをクリーンアップ。
			// 終了情報をログに記録（common.Info）
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
	//リモートデスクトップセッションで発生するイベント（RAW_DATA_ARRIVE, DESKTOP_INIT, DESKTOP_QUIT）を処理します。セッションの初期化や終了、データ転送などを効率的に管理し、エラーや状態を適切に処理することを目的としています。
}

/*
**onDesktopConnect**は、新しいデスクトップセッションが接続された際に呼ばれるハンドラーです。
WebSocket接続時にDeviceが取得できなければ、セッションを閉じてエラーメッセージを送信します。
desktopインスタンスを作成し、セッションに関連付けます。これにより、デスクトップセッションが初期化され、通信が可能になります。
*/
//リモートデスクトップのクライアント接続を初期化する関数です。onDesktopConnect 関数は、新しいデスクトップセッションを作成し、リモートデバイスに関連付けます。また、セッションの初期化やエラーハンドリングを行います。
/*
役割: リモートデスクトップ接続を処理し、クライアント (ブラウザ) とデバイス間の通信を可能にする。
主な処理:
クライアントから送信された接続リクエストを検証。
クライアントが接続する対象のデバイスを確認。
新しいデスクトップセッションを作成。
セッションの初期化イベントをデバイスに送信。
*/
func onDesktopConnect(session *melody.Session) {
	//クライアントの接続情報を検証
	//セッションオブジェクト (session) に保存されているデバイス情報 (Device) を取得。
	// session.Get("Device") はセッション内のデータを取得。
	// 情報が存在しない場合、セッションを閉じてエラー通知をクライアントに送信。
	device, ok := session.Get(`Device`)
	if !ok {
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|DESKTOP.CREATE_SESSION_FAILED}`}, session)
		session.Close()
		return
	}
	// デバイスの存在を確認
	//指定されたデバイスが存在するかを確認。
	// common.CheckDevice(device.(string), ``) はデバイス ID (device) を検索。
	// 存在しない場合、エラー通知をクライアントに送信し、セッションを閉じる。
	connUUID, ok := common.CheckDevice(device.(string), ``)
	if !ok {
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|COMMON.DEVICE_NOT_EXIST}`}, session)
		session.Close()
		return
	}
	//デバイスとの接続を確認
	//デバイス接続 (deviceConn) が有効か確認。
	// common.Melody.GetSessionByUUID(connUUID) を使って、該当するデバイス接続セッションを取得。
	// 接続が無効な場合、エラー通知をクライアントに送信し、セッションを閉じる。
	deviceConn, ok := common.Melody.GetSessionByUUID(connUUID)
	if !ok {
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|COMMON.DEVICE_NOT_EXIST}`}, session)
		session.Close()
		return
	}
	//デスクトップセッションの作成
	//新しいデスクトップセッションを作成。
	// 一意の識別子 (desktopUUID) を生成し、それをセッションに関連付け。
	// desktop オブジェクトは、デスクトップセッションに必要な情報（クライアント接続、デバイス接続、UUID など）を保持。
	// セッションに Desktop キーでデスクトップオブジェクトを設定。
	desktopUUID := utils.GetStrUUID()
	desktop := &desktop{
		uuid:       desktopUUID,
		device:     device.(string),
		srcConn:    session,
		deviceConn: deviceConn,
	}
	session.Set(`Desktop`, desktop)
	//イベントハンドラの登録
	// デスクトップセッションのイベントハンドラを登録。
	// desktopEventWrapper(desktop) は、このセッション専用のイベント処理関数を生成。
	// common.AddEvent を使って、デバイス UUID (connUUID) とデスクトップ UUID (desktopUUID) を関連付け、イベントハンドラを登録。
	common.AddEvent(desktopEventWrapper(desktop), connUUID, desktopUUID)
	//セッション初期化イベントをデバイスに送信
	//デスクトップセッションの初期化イベントをデバイスに通知。
	// modules.Packet は、デバイスに送信するデータパケット。
	// Act: "DESKTOP_INIT" は、デバイス側がセッションを初期化するアクションを表す。
	// Data フィールドには、デスクトップセッションの UUID が含まれる。
	common.SendPack(modules.Packet{Act: `DESKTOP_INIT`, Data: gin.H{
		`desktop`: desktopUUID,
	}, Event: desktopUUID}, deviceConn)
	//接続成功のログを記録
	//接続成功の情報をログに記録。
	// common.Info は、接続に成功したことをログに残します。
	common.Info(desktop.srcConn, `DESKTOP_CONN`, `success`, ``, map[string]any{
		`deviceConn`: desktop.deviceConn,
	})

	/*
		処理の全体的な流れ
		クライアントがリモートデスクトップに接続を試みる。
		デバイス情報を検証し、該当するデバイスの接続を確認。
		デバイスが存在し、接続が有効な場合、新しいデスクトップセッションを作成。
		セッションの初期化イベントをデバイスに送信し、通信を確立。
		セッションやイベントの情報を記録。
	*/
}

/*
**onDesktopMessage**は、デスクトップセッションからのメッセージを処理します。
バイナリパケットの検証: パケットが有効であるかを確認します。無効な場合はセッションを閉じてエラーを返します。
メッセージ内容に応じた処理: 受け取ったパケットのActに応じて、DESKTOP_PINGやDESKTOP_KILL、DESKTOP_SHOTなどの操作を行います。
*/
//リモートデスクトップのクライアント（通常はブラウザ）からのメッセージを処理するための関数です。クライアントがデスクトップ操作や状態確認を要求したときに、それに対応するアクションをデバイスに送信します。
/*
クライアントから送信されたメッセージを解析し、有効なリクエストであれば、デバイスに対応する指示を送る。
無効なリクエストやエラーが発生した場合、セッションを閉じる。
主な処理の流れ:

クライアントセッションからデスクトップ情報を取得。
メッセージのフォーマットと種類を検証。
メッセージのデータを復号し、パケットとして解析。
パケットの内容に基づき、適切なアクションを実行。
*/
func onDesktopMessage(session *melody.Session, data []byte) {
	var pack modules.Packet
	//セッションからデスクトップ情報を取得
	//セッションに関連付けられた Desktop 情報を取得。
	// セッション (session) から Desktop キーで値を取得。
	// デスクトップ情報が見つからない場合は、処理を中断。
	val, ok := session.Get(`Desktop`)
	if !ok {
		return
	}
	desktop := val.(*desktop)

	//メッセージのフォーマットと種類を検証
	// メッセージが正しい形式であり、かつ特定の種類であることを確認。
	// utils.CheckBinaryPack(data) でメッセージを解析し、以下を取得：
	// service: サービスコード（ここでは 20 を期待）。
	// op: 操作コード（ここでは 03 を期待）。
	// isBinary: メッセージがバイナリ形式かどうか。
	// サービスコードが 20 でない、操作コードが 03 でない、またはバイナリ形式でない場合、エラーを返してセッションを閉じる。
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

	//メッセージデータの復号と解析
	//メッセージデータを復号し、JSONとして解析。
	// data[8:] を復号してクライアントのパケットデータ (pack) を取得。
	// 復号後、data を utils.JSON.Unmarshal を使用して pack 構造体に変換。
	// 解析に失敗した場合はエラーを返してセッションを閉じる。
	// 最後にセッションの LastPack を現在の時刻で更新。
	// 	data = utility.SimpleDecrypt(data[8:], session)
	if utils.JSON.Unmarshal(data, &pack) != nil {
		sendPack(modules.Packet{Code: -1}, session)
		session.Close()
		return
	}
	session.Set(`LastPack`, utils.Unix)

	//パケットの内容に基づく処理
	//pack.Act の値に基づいて、適切なアクションを実行。
	// pack.Act でパケットの種類を判別。
	// サポートされる種類と対応する処理：

	switch pack.Act {
	// DESKTOP_PING:
	// デスクトップセッションの存在確認をデバイスに通知。
	case `DESKTOP_PING`:
		common.SendPack(modules.Packet{Act: `DESKTOP_PING`, Data: gin.H{
			`desktop`: desktop.uuid,
		}, Event: desktop.uuid}, desktop.deviceConn)
		return

		// DESKTOP_KILL:
	// セッションの終了をデバイスに通知。
	// クライアントにも終了メッセージを送信。
	// ログを記録。
	case `DESKTOP_KILL`:
		common.Info(desktop.srcConn, `DESKTOP_KILL`, `success`, ``, map[string]any{
			`deviceConn`: desktop.deviceConn,
		})
		common.SendPack(modules.Packet{Act: `DESKTOP_KILL`, Data: gin.H{
			`desktop`: desktop.uuid,
		}, Event: desktop.uuid}, desktop.deviceConn)
		return

		// DESKTOP_SHOT:
	// デスクトップのスクリーンショット要求をデバイスに送信。
	// サポートされない種類の場合、セッションを閉じる。
	case `DESKTOP_SHOT`:
		common.SendPack(modules.Packet{Act: `DESKTOP_SHOT`, Data: gin.H{
			`desktop`: desktop.uuid,
		}, Event: desktop.uuid}, desktop.deviceConn)
		return
	}
	session.Close()

	/*
		接続セッションの確認:
		セッションがデスクトップに関連付けられているか確認。
		メッセージ検証:
		メッセージが正しい形式で送信されているか確認。
		データ復号と解析:
		クライアントから送信されたデータを復号し、パケット構造体として解析。
		リクエスト処理:
		パケットの種類に基づき、デバイスとの通信を行い、指示や通知を送信。

		onDesktopMessage 関数は、クライアントからのメッセージを解析し、デバイスに適切なアクションを伝える橋渡しの役割を果たします。このような設計により、クライアントとデバイス間の非同期通信を効率的に処理できます。
	*/
}

/*
**onDesktopDisconnect**は、デスクトップセッションが切断された際に呼ばれます。
セッション切断時に、デバイスにセッションが終了したことを通知し、イベントやセッション情報をクリアします。
*/
//WebSocketセッションが切断された際に実行される処理を定義しています。この処理は、リモートデスクトップセッションが切断されたときに関連リソースを適切にクリーンアップし、デバイスに通知を送る役割を果たします。
func onDesktopDisconnect(session *melody.Session) {
	//セッション切断ログの記録
	//セッションの切断が発生したことをログに記録します。
	// DESKTOP_CLOSE イベントとして成功ログ (success) を記録。
	// session がどのセッションであるかを指定。
	common.Info(session, `DESKTOP_CLOSE`, `success`, ``, nil)
	//デスクトップ情報の取得
	//セッションに関連付けられている Desktop 情報を取得します。
	// session.Get("Desktop") でデスクトップ情報を取得。
	// 値が存在しない場合、または型が正しくない場合 (*desktop にキャストできない場合)、処理を終了。
	// 結果:
	// 有効なデスクトップ情報を取得できた場合、後続の処理を続行。
	val, ok := session.Get(`Desktop`)
	if !ok {
		return
	}
	desktop, ok := val.(*desktop)
	if !ok {
		return
	}
	//デバイスへの通知
	//セッション終了をデバイスに通知します。
	// modules.Packet を作成し、DESKTOP_KILL アクションを設定。
	// desktop.uuid を Data に含めてデバイスに送信。
	// Event フィールドにも desktop.uuid を設定。
	// 結果:
	// デバイス側で、この通知を受け取り、デスクトップセッション終了の処理を行う。

	common.SendPack(modules.Packet{Act: `DESKTOP_KILL`, Data: gin.H{
		`desktop`: desktop.uuid,
	}, Event: desktop.uuid}, desktop.deviceConn)

	//イベントの削除
	//セッションに関連付けられたイベントハンドラを削除します。
	// セッションの uuid を指定してイベントを削除。
	common.RemoveEvent(desktop.uuid)

	//セッションとデスクトップ情報のクリーンアップ
	//セッションとデスクトップ情報をクリーンアップし、メモリを解放します。
	// セッションから Desktop を削除 (nil を設定)。
	// desktop 変数を明示的に nil にして不要な参照を削除。
	session.Set(`Desktop`, nil)
	desktop = nil

	/*
		処理の全体的な流れ
		セッション切断のログ記録:
		セッションが閉じられたことを記録。
		デスクトップ情報の取得:
		セッションに関連付けられているデスクトップ情報を取得。
		情報が無効であれば終了。
		デバイスへの通知:
		セッション終了をデバイスに通知し、適切に処理させる。
		イベントの削除:
		セッションに関連付けられていたイベントハンドラを削除。
		クリーンアップ:
		セッションとデスクトップ情報を削除し、メモリを解放。
		関数の目的
		onDesktopDisconnect 関数は、デスクトップセッションが切断された際のリソース管理とデバイス通知を担っています。この処理により、不要なイベントやセッションが残ることを防ぎ、切断された状態を正確にデバイスに反映できます。
	*/
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
//特定のセッション (melody.Session) に対してデータ (modules.Packet) を送信するための関数です。データは暗号化され、バイナリ形式で送信されます。
/*
関数の目的
sendPack 関数の目的は、以下を行うことです：

modules.Packet を JSON データに変換。
セッション固有の暗号化キーを用いてデータを暗号化。
特定のフォーマットに従ったバイナリデータとしてセッションに送信。
送信が成功したかどうかを返す。
*/
func sendPack(pack modules.Packet, session *melody.Session) bool {
	//セッションの確認
	//session が nil（無効）である場合は、処理を中断して false を返します。
	// 有効なセッションでなければ送信できないため。
	if session == nil {
		return false
	}
	//パケットのシリアライズ
	//modules.Packet 構造体を JSON 形式のバイト配列に変換。
	// シリアライズに失敗した場合（err != nil）、処理を中断して false を返します。
	data, err := utils.JSON.Marshal(pack)
	if err != nil {
		return false
	}
	//データの暗号化
	//暗号化されたデータを utility.SimpleEncrypt 関数で生成。
	// 暗号化には、セッション固有の情報（暗号化キーなど）が使用されていると考えられます。
	// 暗号化により、セッション間でデータが安全に送信されることを保証します。
	data = utility.SimpleEncrypt(data, session)

	//データ送信
	//session.WriteBinary:
	// 暗号化されたデータをセッションにバイナリ形式で送信。
	// 送信するデータには特定のヘッダー []byte{34, 22, 19, 17, 20, 03} を付加します。
	// ヘッダーはプロトコルを識別するためのシグネチャとして機能します。
	// 34, 22, 19, 17, 20, 03 は固定の6バイトで、データの形式や送信元/宛先を示すメタ情報。
	// 送信結果（エラーが発生したかどうか）をチェックし、成功であれば true、失敗であれば false を返します。
	err = session.WriteBinary(append([]byte{34, 22, 19, 17, 20, 03}, data...))
	return err == nil

	/*
		関数全体の流れ
		セッションが有効か確認：
		セッションが無効なら即終了。
		データを JSON に変換：
		パケット構造体をシリアライズ。
		シリアライズ失敗時に終了。
		データを暗号化：
		JSON データを暗号化。
		バイナリデータ送信：
		ヘッダーを付加して暗号化データをセッションに送信。
		送信が成功したかどうかを返す。


		セッションに対して安全かつプロトコルに準拠した方法でデータを送信することを目的としています。データの暗号化と送信が組み合わさり、セッションベースの通信での機密性を保つ役割を果たします。
	*/
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

// 指定された deviceID に関連するすべてのデスクトップセッションを終了する関数です。これにより、特定のデバイスに紐づいた WebSocket セッションを安全に閉じることができます。
/*
関数の目的
セッションの特定:
指定されたデバイス ID (deviceID) に紐づいたすべてのデスクトップセッションを特定します。
セッションの終了:
特定されたセッションに対して終了パケットを送信し、その後セッションをクローズします。
*/
func CloseSessionsByDevice(deviceID string) {
	//セッションキューの準備
	//終了するセッションを一時的に保存するスライス queue を作成します。
	var queue []*melody.Session

	//セッションの反復処理
	//desktopSessions.IterSessions を使って、現在のすべてのセッションを反復処理します。
	// セッションごとに以下のチェックを行います。
	desktopSessions.IterSessions(func(_ string, session *melody.Session) bool {
		//デスクトップセッションの確認
		//セッションから Desktop 属性を取得します。
		// Desktop 属性がない、または型が正しくない場合は次のセッションに進みます（return true）。
		val, ok := session.Get(`Desktop`)
		if !ok {
			return true
		}
		desktop, ok := val.(*desktop)
		if !ok {
			return true
		}
		//デバイス ID の一致確認
		//Desktop 属性の device が指定された deviceID と一致するか確認します。
		//一致する場合:
		if desktop.device == deviceID {
			//終了パケットの送信
			sendPack(modules.Packet{Act: `QUIT`, Msg: `${i18n|DESKTOP.SESSION_CLOSED}`}, desktop.srcConn)

			//セッションをキューに追加
			//対象セッションを終了するためのキューに追加します。
			queue = append(queue, session)
			//return false によって反復処理を終了します。
			return false
		}
		//return true によって次のセッションに進みます。
		return true
	})

	//セッションの終了
	// すべての終了対象セッションを閉じます。
	// session.Close():
	// セッションを安全に終了し、関連リソースを解放します。
	for _, session := range queue {
		session.Close()
	}

	/*
		実行の流れ
		全セッションをチェック:
		現在存在するすべてのセッションを確認します。
		終了対象を特定:
		指定された deviceID に関連付けられたセッションを特定し、終了対象としてキューに追加します。
		終了通知の送信:
		特定されたセッションに対し、終了の通知 (QUIT パケット) を送信します。
		セッションの閉鎖:
		キューに格納されたセッションを実際に閉じます。

		特定のデバイス ID に関連するすべてのデスクトップセッションを安全かつ確実に閉じるためのロジックを提供します。セッションを閉じる前に通知を送信し、クライアントとサーバーの状態を同期させる仕組みが実装されています。
	*/
}
