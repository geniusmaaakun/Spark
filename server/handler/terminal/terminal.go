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

// terminalSessions は、リモートデバイスとブラウザ間のWebSocketセッションを管理するための melody ライブラリを使用しています。
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
//ターミナルセッションを初期化するためのWebSocketリクエストを処理する関数です。クライアントがWebSocketを使ってデバイスと通信するターミナルをセットアップする際に使用されます。
func InitTerminal(ctx *gin.Context) {
	//WebSocketリクエストの確認
	//リクエストがWebSocketでない場合は処理を中止し、HTTP 400 (Bad Request) を返します。
	// 理由: このエンドポイントはWebSocket通信専用であり、HTTPなど他のプロトコルでは動作しません。
	if !ctx.IsWebsocket() {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	//必要なクエリパラメータの取得と検証
	//クライアントから送信された secret パラメータを取得します。
	//secret は必須であり、長さは32文字（16バイトの16進数表現）である必要があります。
	//指定された secret をデコードしてバイナリ形式に変換。
	//不正な形式（例: 長さが異なる、16進数として無効など）の場合はエラーを返して終了。
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
	//secret の目的
	//クライアントの認証情報として使用。
	//クエリパラメータ device を取得します。
	//device は必須です。
	//ターミナルセッションを開始する対象デバイスを特定します。
	device, ok := ctx.GetQuery(`device`)
	if !ok {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
	// デバイスの存在確認
	//指定された device が現在接続されているデバイス一覧に存在するか確認します。
	if _, ok := common.CheckDevice(device, ``); !ok {
		//デバイスが存在しない場合は、HTTP 400 (Bad Request) を返して終了。
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}

	//ターミナルセッションのハンドリング
	//WebSocketリクエストを処理し、ターミナルセッションを開始します。
	// HandleRequestWithKeys は、WebSocketのリクエストを処理しつつ、セッションに関連付けるキーやデータを登録します。
	// セッションに登録するデータ:
	// Secret: クライアントが送信した認証用のシークレット。
	// Device: セッションが紐づくデバイスID。
	// LastPack: セッションの最後のアクティビティ時刻。
	terminalSessions.HandleRequestWithKeys(ctx.Writer, ctx.Request, gin.H{
		`Secret`:   secret,
		`Device`:   device,
		`LastPack`: utils.Unix,
	})

	/*
		動作のまとめ
		クライアントがターミナルセッションを要求:

		WebSocketプロトコルでリクエストを送信。
		必要なパラメータ (secret と device) を含む。
		パラメータの検証:

		secret の形式（長さと内容）をチェック。
		device が存在するか確認。
		セッションの初期化:

		検証に成功した場合、指定されたデバイスに対してターミナルセッションを開始。
		セッション情報は terminalSessions に登録される。



		使用例
		クライアント側リクエスト例:

		arduino
		コードをコピーする
		ws://example.com/terminal?secret=0123456789abcdef0123456789abcdef&device=12345
		secret: 認証情報。
		device: ターミナルを操作する対象デバイスのID。
		成功時:

		ターミナルセッションが初期化され、WebSocket通信が確立される。
		失敗時:

		必要なパラメータが欠如、または検証エラーの場合、HTTP 400 エラーを返す。
	*/
}

/*
この関数はターミナルイベントのラッパーです。リモートデバイスからターミナルにデータが送信された場合、そのデータを処理してブラウザに返します。
TERMINAL_INIT や TERMINAL_OUTPUT などのイベントに応じて処理を分岐させます。
*/
// terminalEventWrapper returns a eventCallback function that will
// be called when device need to send a packet to browser
//ターミナルセッションのイベントを処理するための EventCallback を生成する関数です。ターミナルセッションに関連する特定のイベント（初期化、終了、データ送受信）をハンドリングするロジックが含まれています。
func terminalEventWrapper(terminal *terminal) common.EventCallback {
	return func(pack modules.Packet, device *melody.Session) {
		//イベントのデータ検証と処理

		// イベントデータの検証と処理
		//RAW_DATA_ARRIVE イベントを特別に処理。
		//データの復号化やJSON解析を行います。
		if pack.Act == `RAW_DATA_ARRIVE` && pack.Data != nil {
			// dataを取り出す
			data := *pack.Data[`data`].(*[]byte)

			//data[5] == 00: バイナリデータをそのままWebSocketセッションに転送。
			if data[5] == 00 {
				terminal.session.WriteBinary(data)
				return
			}

			//その他の値の場合は無視。
			if data[5] != 01 {
				return
			}

			//data[5] == 01: 暗号化されたデータを復号化して解析。
			// 8番目の要素　以降を取り出す
			data = data[8:]

			//utility.SimpleDecrypt(data, device) を使ってデータを復号化。
			data = utility.SimpleDecrypt(data, device)
			//復号化後のデータを modules.Packet として解析。
			if utils.JSON.Unmarshal(data, &pack) != nil {
				return
			}
		}

		//ターミナルイベントの処理
		//TERMINAL_INIT: ターミナルセッションの初期化結果を処理。
		switch pack.Act {
		case `TERMINAL_INIT`:
			// 0でない場合は失敗
			if pack.Code != 0 {
				// メッセージを追加
				msg := `${i18n|TERMINAL.CREATE_SESSION_FAILED}`
				if len(pack.Msg) > 0 {
					msg += `: ` + pack.Msg
				} else {
					msg += `${i18n|COMMON.UNKNOWN_ERROR}`
				}
				//クライアントに終了通知 (QUIT パケット) を送信。
				sendPack(modules.Packet{Act: `QUIT`, Msg: msg}, terminal.session)

				//イベントを削除し、セッションを閉じる。
				common.RemoveEvent(terminal.uuid)
				terminal.session.Close()

				//ログに失敗情報を記録。
				common.Warn(terminal.session, `TERMINAL_INIT`, `fail`, msg, map[string]any{
					`deviceConn`: terminal.deviceConn,
				})
				// 成功
			} else {
				//成功情報をログに記録。
				common.Info(terminal.session, `TERMINAL_INIT`, `success`, ``, map[string]any{
					`deviceConn`: terminal.deviceConn,
				})
			}

			//TERMINAL_QUIT: セッションの終了処理。
		case `TERMINAL_QUIT`:
			//終了メッセージを生成。
			msg := `${i18n|TERMINAL.SESSION_CLOSED}`
			if len(pack.Msg) > 0 {
				msg = pack.Msg
			}
			//クライアントに終了通知 (QUIT パケット) を送信。
			sendPack(modules.Packet{Act: `QUIT`, Msg: msg}, terminal.session)
			//イベントを削除し、セッションを閉じる。
			common.RemoveEvent(terminal.uuid)
			terminal.session.Close()
			common.Info(terminal.session, `TERMINAL_QUIT`, ``, msg, map[string]any{
				`deviceConn`: terminal.deviceConn,
			})

			//TERMINAL_OUTPUT: デバイスから送信されたターミナルの出力データをクライアントに転送。
		case `TERMINAL_OUTPUT`:
			//pack.Data に出力データが含まれていることを確認。
			if pack.Data == nil {
				return
			}
			//ターミナル出力データをクライアントに転送。
			if output, ok := pack.Data[`output`]; ok {
				//データを TERMINAL_OUTPUT パケットとしてクライアントに送信。
				sendPack(modules.Packet{Act: `TERMINAL_OUTPUT`, Data: gin.H{
					`output`: output,
				}}, terminal.session)
			}
		}
	}

	/*
			イベントデータの処理:
		RAW_DATA_ARRIVE イベントで暗号化データを復号化・解析。
		イベントの種類に応じた処理:
		TERMINAL_INIT: セッション初期化の成否を処理。
		TERMINAL_QUIT: セッション終了処理。
		TERMINAL_OUTPUT: ターミナル出力データの転送。
		この仕組みは、クライアントとデバイス間でのターミナルセッションの制御とデータ通信を効率的に行うために設計されています。
	*/

	/*
					i18n は "internationalization"（国際化） の略語です。
							i nternationalizatio n

		i18n の関連用語:
			l10n（localization / ローカリゼーション）:
				対象地域や文化に合わせて、特定の言語や形式を適用するプロセス。
				例: 日付の表示形式を「YYYY/MM/DD」から「MM/DD/YYYY」に変更。
			g11n（globalization / グローバリゼーション）:
				i18n と l10n の両方を含む、システム全体を多言語・多地域対応にするプロセス。
		意味と由来
			「i18n」という略語は、単語の最初の文字 "i" と最後の文字 "n" の間に18個の文字があることから来ています。

		実際の用途
			テキストの翻訳対応

		アプリケーション内のメッセージやテキストを複数の言語で表示できるようにする。
		例: 「エラーが発生しました」→ 「An error occurred」など。
		ローカリゼーションの準備

		日付形式、数値形式、通貨形式、住所など、地域ごとの違いに対応する。
		マルチリンガルサポート

		ユーザーの選んだ言語や地域に基づいて、適切なメッセージやデータを表示。
		このコードでの i18n の役割
		${i18n|...} の形で、翻訳キーが記述されています。

		例: ${i18n|TERMINAL.CREATE_SESSION_FAILED}
		このキーは「ターミナルセッションの作成に失敗しました」というエラーメッセージを、言語設定に基づいて適切な翻訳に置き換えるためのものです。
		このようなキーは、翻訳ファイル（JSONやYAML形式など）で実際の翻訳テキストと関連付けられていることが多いです。

		例:
		json
		コードをコピーする
		{
		  "TERMINAL": {
		    "CREATE_SESSION_FAILED": "ターミナルセッションの作成に失敗しました"
		  }
		}
	*/
}

/*
WebSocket接続が確立された際に呼び出されるコールバック関数です。
デバイスの存在を確認し、セッションを初期化します。
*/
//WebSocket セッションが新しく接続された際に呼び出されます。
// 接続リクエストが有効かどうかを確認し、指定されたデバイスに対してターミナルセッションを作成し、デバイスに初期化メッセージを送信します。
func onTerminalConnect(session *melody.Session) {
	//デバイス情報の取得
	//セッションオブジェクト (session) から Device キーを取得します。
	device, ok := session.Get(`Device`)
	if !ok {
		//デバイス情報が存在しない場合（ok == false）、エラーメッセージをクライアントに送信して接続を終了します。
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|TERMINAL.CREATE_SESSION_FAILED}`}, session)
		session.Close()
		return
	}

	//デバイスの存在確認
	//common.CheckDevice を呼び出し、指定されたデバイス ID が既知のデバイスリストに存在するか確認します。
	connUUID, ok := common.CheckDevice(device.(string), ``)
	if !ok {
		// 存在しない場合はエラーメッセージを送信して接続を終了します。
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|COMMON.DEVICE_NOT_EXIST}`}, session)
		session.Close()
		return
	}

	//デバイス接続セッションの取得
	//common.Melody.GetSessionByUUID を使用して、デバイスに対応する WebSocket セッションを取得します。
	deviceConn, ok := common.Melody.GetSessionByUUID(connUUID)
	if !ok {
		//デバイスのセッションが存在しない場合、エラーメッセージを送信して接続を終了します。
		sendPack(modules.Packet{Act: `WARN`, Msg: `${i18n|COMMON.DEVICE_NOT_EXIST}`}, session)
		session.Close()
		return
	}

	//ターミナルセッションの初期化
	//ターミナルセッション用の一意な ID を生成します。
	uuid := utils.GetStrUUID()
	//terminal 構造体を作成し、デバイス ID、セッション、デバイス接続情報などを格納します。
	terminal := &terminal{
		uuid:       uuid,
		device:     device.(string),
		session:    session,
		deviceConn: deviceConn,
	}
	//セッションに Terminal キーとしてこのターミナルセッション情報を設定します。
	session.Set(`Terminal`, terminal)

	//イベントハンドラーの登録
	//ターミナルセッションに関連付けられたイベントハンドラーを登録します。
	//terminalEventWrapper は、ターミナル操作やデータ処理を行うためのコールバック関数です。
	common.AddEvent(terminalEventWrapper(terminal), connUUID, uuid)

	//デバイスに初期化メッセージを送信
	//デバイスに対して TERMINAL_INIT アクションを含むパケットを送信します。
	//パケットにはターミナルセッションの UUID が含まれており、デバイス側で対応する処理が行われます。
	common.SendPack(modules.Packet{Act: `TERMINAL_INIT`, Data: gin.H{
		`terminal`: uuid,
	}, Event: uuid}, deviceConn)
	//ログ記録
	//ターミナル接続が正常に初期化されたことをログに記録します。
	common.Info(terminal.session, `TERMINAL_CONN`, `success`, ``, map[string]any{
		`deviceConn`: terminal.deviceConn,
	})

	/*
		エラーハンドリングの流れ
		各段階で失敗が発生した場合、適切なエラーメッセージ（国際化対応済み）を送信し、セッションを閉じます。
		主なエラー条件：
		デバイス情報がセッションにない。
		デバイスが未登録または存在しない。
		デバイスの接続セッションが見つからない。


		動作の概要
		クライアントが WebSocket 経由で接続リクエストを送信。
		サーバーがデバイスの検証とターミナルセッションの初期化を実施。
		デバイスに初期化パケットを送信して、ターミナルセッションを確立。
	*/
}

/*
WebSocket経由で受信したメッセージを処理します。
バイナリメッセージかどうかを確認し、適切に処理を振り分けます。
*/
//WebSocket セッション (melody.Session) を通じてターミナルからのメッセージを処理する関数です。受信したデータを解析し、適切なアクションを実行する役割を果たします。
func onTerminalMessage(session *melody.Session, data []byte) {

	//セッションからターミナル情報を取得
	var pack modules.Packet
	//session に関連付けられたターミナル情報 (terminal) を取得します。
	val, ok := session.Get(`Terminal`)
	//情報が存在しない場合は、処理を終了します。
	if !ok {
		return
	}

	//データ形式と操作コードの検証
	terminal := val.(*terminal)

	//受信データがバイナリ形式であるか (isBinary) を確認。
	service, op, isBinary := utils.CheckBinaryPack(data)

	//service がターミナル操作を示す 21 であるかをチェック。
	//条件を満たさない場合、エラーコードを返し、セッションを閉じます。
	if !isBinary || service != 21 {
		sendPack(modules.Packet{Code: -1}, session)
		session.Close()
		return
	}

	//RAW データの処理
	//操作コード (op) が 00 の場合、受信したデータはそのままデバイス側に転送されます。
	if op == 00 {
		// 時間を設定
		session.Set(`LastPack`, utils.Unix)
		//terminal.uuid をデータに付加し、フォーマットを整えた上で転送します。
		rawEvent, _ := hex.DecodeString(terminal.uuid)
		data = append(data, rawEvent...)
		copy(data[22:], data[6:])
		copy(data[6:], rawEvent)
		terminal.deviceConn.WriteBinary(data)
		return
	}

	//無効な操作コードの処理
	//op が 01 以外の場合、エラーコードを返し、セッションを閉じます。
	if op != 01 {
		sendPack(modules.Packet{Code: -1}, session)
		session.Close()
		return
	}

	//データをデコードし、メッセージを解析
	//データをデコードし (SimpleDecrypt)、JSON形式に変換 (Unmarshal) します。
	data = utility.SimpleDecrypt(data[8:], session)
	//デコードに失敗した場合、エラーを返しセッションを閉じます
	if utils.JSON.Unmarshal(data, &pack) != nil {
		sendPack(modules.Packet{Code: -1}, session)
		session.Close()
		return
	}
	//データが正常であれば、セッションの最終パケット時刻 (LastPack) を更新します。
	session.Set(`LastPack`, utils.Unix)

	//メッセージ内容に基づく処理
	switch pack.Act {
	//input フィールドのデータを取得。
	case `TERMINAL_INPUT`:
		if pack.Data == nil {
			return
		}
		//デコードしたコマンドを terminal.deviceConn に転送。
		if input, ok := pack.GetData(`input`, reflect.String); ok {
			rawInput, _ := hex.DecodeString(input.(string))
			//ログに入力内容 (rawInput) を記録。
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

	//ターミナルのサイズ変更 (cols, rows) をデバイスに通知。
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

	//ターミナルセッションを終了する命令をデバイスに送信。
	case `TERMINAL_KILL`:
		common.Info(terminal.session, `TERMINAL_KILL`, `success`, ``, map[string]any{
			`deviceConn`: terminal.deviceConn,
		})
		common.SendPack(modules.Packet{Act: `TERMINAL_KILL`, Data: gin.H{
			`terminal`: terminal.uuid,
		}, Event: terminal.uuid}, terminal.deviceConn)
		return

	//ターミナルの接続を維持するための PING をデバイスに送信。
	case `PING`:
		common.SendPack(modules.Packet{Act: `TERMINAL_PING`, Data: gin.H{
			`terminal`: terminal.uuid,
		}, Event: terminal.uuid}, terminal.deviceConn)
		return
	}

	//対応していない操作の場合、セッションを閉じます。
	session.Close()

	/*
			要点まとめ
		クライアントから送信されたターミナルデータを検証し、デバイスに適切に転送。
		操作コードやメッセージ内容 (Act) に基づいて特定の処理を実行。
		エラーや無効なデータの場合は、セッションを閉じ、エラーを通知。
	*/
}

/*
WebSocketが切断された際に呼び出されます。
セッションのクリーンアップを行い、関連するリソースを解放します。
*/
//WebSocket セッションが切断された際に実行される「ターミナルの後処理」を担当します。具体的には、関連するターミナル情報を削除し、デバイスにセッション終了を通知します。
func onTerminalDisconnect(session *melody.Session) {
	//ログ出力
	//セッションが切断されたことをログに記録します。
	// ログの種類は「情報」(Info) で、TERMINAL_CLOSE というイベント名を使用しています。
	// 成功 (success) として記録し、特に追加のメッセージ (msg) はありません。
	common.Info(session, `TERMINAL_CLOSE`, `success`, ``, nil)
	val, ok := session.Get(`Terminal`)
	if !ok {
		return
	}
	//セッションからターミナル情報を取得
	//session からターミナル情報 (Terminal) を取得します。
	// ターミナル情報がない場合、または型が正しくない場合、処理を終了します。
	terminal, ok := val.(*terminal)
	if !ok {
		return
	}

	//デバイスにターミナル終了を通知
	//デバイス (terminal.deviceConn) に対して、ターミナル終了 (TERMINAL_KILL) を通知します。
	//modules.Packet を使用して、以下のデータを送信します
	//Act: 動作を示す識別子。ここでは TERMINAL_KILL。
	// Data: データペイロード。ターミナルの識別子 (terminal.uuid) を含む。
	// Event: イベント識別子。ここではターミナルの UUID。
	common.SendPack(modules.Packet{Act: `TERMINAL_KILL`, Data: gin.H{
		`terminal`: terminal.uuid,
	}, Event: terminal.uuid}, terminal.deviceConn)
	// イベントリスナーの削除
	//このターミナルセッションに関連付けられたイベントリスナーを削除します。
	// イベントは、ターミナルの UUID をキーとして管理されています。
	common.RemoveEvent(terminal.uuid)

	//セッション情報のクリア
	//セッションから Terminal に関連する情報を削除します。
	// メモリ解放のために terminal を明示的に nil に設定。
	session.Set(`Terminal`, nil)
	terminal = nil

	/*
			まとめ
		この関数は、ターミナルセッションのクリーンアップを行う重要な処理です。具体的には次の操作を行います：

		セッション終了をログに記録。
		セッションからターミナル情報を取得。
		ターミナル終了 (TERMINAL_KILL) をデバイスに通知。
		ターミナルに関連するイベントリスナーを削除。
		セッション情報をクリア。
	*/
}

// ターミナルセッションにデータを送信するための関数です。
// 指定されたデータパケット（modules.Packet）を暗号化し、WebSocketセッション（melody.Session）を通じてバイナリ形式で送信する関数です。
/*
目的:
	クライアントに対してデータ（modules.Packet）を送信する。
	データは暗号化された上で送信されます。
引数:
	pack modules.Packet: 送信するデータパケット。
	session *melody.Session: データを送信するための WebSocket セッション。
戻り値: bool
	true: 送信が成功した場合。
	false: エラーが発生した場合
*/
func sendPack(pack modules.Packet, session *melody.Session) bool {
	//セッションの有無を確認
	//WebSocket セッションが nil（存在しない場合）であれば、送信を行わず false を返す。
	// セッションが無効であれば、送信できないためこのチェックを行う。
	if session == nil {
		return false
	}
	//データを JSON にシリアライズ
	//modules.Packet 構造体を JSON フォーマットに変換します。
	// この形式変換は、WebSocket でのデータ送信が標準的に扱える形式にするため。
	// エラー処理:
	// JSON 変換に失敗した場合（例えば不正なデータ構造など）、false を返す。
	data, err := utils.JSON.Marshal(pack)
	if err != nil {
		return false
	}
	//データを暗号化
	//シリアライズされた JSON データを暗号化します。
	// 暗号化の手法は utility.SimpleEncrypt に依存しています。
	// 一般的に、この手法によりデータが転送途中で盗聴されたり改ざんされたりするリスクを軽減します。
	// 暗号化にはセッション情報（session）が関与するため、セッションごとに異なる暗号化が適用される可能性があります。
	data = utility.SimpleEncrypt(data, session)
	//データを WebSocket セッションで送信
	//暗号化されたデータを WebSocket セッションで送信します。
	// 送信形式: バイナリデータ。
	// エラー処理:
	// session.WriteBinary がエラーを返した場合、送信失敗として処理。
	err = session.WriteBinary(data)

	//結果を返す
	// データ送信の成否に基づき、以下の値を返します：
	// 成功 (err == nil): true
	// 失敗 (err != nil): false
	return err == nil

	/*
			動作の流れ
		セッションの有効性を確認。
		modules.Packet を JSON に変換。
		データを暗号化。
		暗号化されたデータを WebSocket セッションで送信。
		成功または失敗を返す。
	*/
}

// 指定されたデバイスIDに関連するすべてのターミナルセッションを閉じます。
// 特定の deviceID に関連付けられたすべてのターミナルセッションを閉じる（終了する）機能を提供します。
/*
目的: 指定された deviceID に関連するすべてのターミナルセッションを検索し、それらを閉じる。
引数:
deviceID string: 閉じたいデバイスの一意の識別子。
戻り値: なし。
*/
func CloseSessionsByDevice(deviceID string) {
	//セッションキューの初期化
	//対象デバイスに関連付けられたセッションを後で閉じるため、セッションの参照を格納するキュー（スライス）を定義。
	var queue []*melody.Session

	//ターミナルセッションを反復
	//terminalSessions.IterSessions:
	// すべてのセッションに対してコールバック関数を実行します。
	// コールバックの戻り値が true の場合は次のセッションへ進み、false の場合は反復を終了します。
	terminalSessions.IterSessions(func(_ string, session *melody.Session) bool {
		//セッションから Terminal 情報を取得
		val, ok := session.Get(`Terminal`)
		if !ok {
			return true
		}
		//セッションに Terminal オブジェクトが関連付けられているかを確認。
		// 関連付けられていない場合: 次のセッションへ進む（return true）。

		//Terminal の型チェック
		//session.Get で取得した値が *terminal 型かどうかを確認。
		// 型が一致しない場合: 次のセッションへ進む（return true）。
		terminal, ok := val.(*terminal)
		if !ok {
			return true
		}
		// deviceID の一致を確認
		//terminal.device が指定された deviceID と一致するかを確認。
		// 一致する場合:
		// 対応するセッションをキューに追加。
		// 反復を終了（return false）。
		// 一致しない場合: 次のセッションへ進む（return true）。
		if terminal.device == deviceID {
			queue = append(queue, session)
			return false
		}
		return true
	})

	//対象セッションを閉じる
	//キューに追加されたすべてのセッションに対して Close メソッドを呼び出し、セッションを閉じます。
	// セッションを閉じる効果:
	// ターミナルセッションが無効化され、リソースが解放される。
	for _, session := range queue {
		session.Close()
	}

	/*
			動作の流れ
		terminalSessions.IterSessions を使用して、すべてのセッションを反復処理。
		各セッションで以下を確認:
		Terminal 情報が存在するか。
		Terminal のデバイス ID が指定された deviceID と一致するか。
		一致するセッションをキューに追加。
		反復終了後、キュー内のすべてのセッションを閉じる。
	*/
}
