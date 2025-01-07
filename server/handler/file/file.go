package file

import (
	"Spark/modules"
	"Spark/server/common"
	"Spark/server/handler/bridge"
	"Spark/server/handler/utility"
	"Spark/utils"
	"Spark/utils/melody"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

/*
Goを使ったファイル管理のためのAPIを実装しており、リモートデバイスとブラウザの間でファイル操作（削除、リスト取得、ファイル取得、アップロードなど）を行うための機能を提供しています。
以下では、各関数の役割とその処理内容について解説します。


リモートデバイスとブラウザ間でのファイル操作を行うためのAPIを実装しています。Ginを使ったHTTPリクエスト処理、Melodyを使ったWebSocket通信、ファイルのアップロード、ダウンロード、削除など、様々なファイル操作に対応しています。また、エラーハンドリングやタイムアウト処理も適切に行われており、信頼性の高い通信を提供します。
*/

/*
目的: リモートデバイス上のファイルを削除します。
処理内容:
リクエストからファイルリストを取得し、指定されたファイルを削除するリクエストをデバイスに送信します。
デバイスからの応答を待ち、削除が成功したかどうかを確認します。
タイムアウト（5秒間応答がない）時には、504 Gateway Timeoutを返します。
*/
// RemoveDeviceFiles will try to get send a packet to
// client and let it upload the file specified.
//リモートデバイス上の指定されたファイルを削除するための操作を処理します。この操作は、特定のデバイスにリクエストを送信し、その応答を確認することで実現されます。
func RemoveDeviceFiles(ctx *gin.Context) {
	//リクエストデータのバインド
	//クライアントから送信されたリクエストデータを form 構造体にバインドします。
	//フィールド Files は、削除対象のファイルパスの配列です。
	var form struct {
		Files []string `json:"files" yaml:"files" form:"files" binding:"required"`
	}
	//リクエストデータの検証を行い、バインドできた場合に ok = true を返します。
	target, ok := utility.CheckForm(ctx, &form)
	//検証に失敗した場合、関数は終了します。
	if !ok {
		return
	}
	//ファイルリストの確認
	//Files 配列が空でないか確認します。
	if len(form.Files) == 0 {
		//空の場合はクライアントにエラーレスポンス (400 Bad Request) を返し、処理を終了します。
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	//リクエストの識別用に UUID (trigger) を生成します。
	trigger := utils.GetStrUUID()
	// リモートデバイスへの削除リクエスト
	/*
		リクエストパケット (modules.Packet) を作成します。
		Act: FILES_REMOVE:
			削除操作を表すアクション名。
		Data: gin.H{files: form.Files}:
			削除対象のファイルリストを含むデータ。
		Event: trigger:
			このリクエストに対応する応答を識別するためのトリガー。
	*/
	//リクエストの送信: common.SendPackByUUID を使用して、ターゲットデバイスにリクエストを送信します。
	common.SendPackByUUID(modules.Packet{Act: `FILES_REMOVE`, Data: gin.H{`files`: form.Files}, Event: trigger}, target)

	//応答イベントの処理
	/*
		応答イベントの登録:
		common.AddEventOnce を使用して、削除リクエストに対応する応答を待ちます。
		応答は trigger を基に識別されます。
		応答待ちのタイムアウトは 5 秒 (5*time.Second) に設定されています。
	*/
	ok = common.AddEventOnce(func(p modules.Packet, _ *melody.Session) {
		/*
			応答の処理:
			応答パケット (modules.Packet) を受け取ると、Code フィールドで結果を判定します。
			失敗 (Code != 0):
			エラーメッセージをログに記録し、クライアントに 500 Internal Server Error を返します。
			成功 (Code == 0):
			成功メッセージをログに記録し、クライアントに 200 OK を返します。
		*/
		if p.Code != 0 {
			common.Warn(ctx, `REMOVE_FILES`, `fail`, p.Msg, map[string]any{
				`files`: form.Files,
			})
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		} else {
			common.Info(ctx, `REMOVE_FILES`, `success`, ``, map[string]any{
				`files`: form.Files,
			})
			ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
		}
	}, target, trigger, 5*time.Second)

	//タイムアウト処理
	//応答がタイムアウトした場合:
	//エラーログを記録し、クライアントに 504 Gateway Timeout を返します。
	if !ok {
		common.Warn(ctx, `REMOVE_FILES`, `fail`, `timeout`, map[string]any{
			`files`: form.Files,
		})
		ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
	}

	/*
		実行の流れ
		リクエストデータの検証:
		クライアントから受け取ったファイルリスト (Files) を検証します。
		リモートデバイスへの削除リクエスト:
		指定されたファイルリストをターゲットデバイスに送信します。
		応答の処理:
		ターゲットデバイスからの応答を受け取り、削除が成功したかを確認します。
		クライアントへのレスポンス:
		成功した場合は 200 OK を、失敗またはタイムアウトした場合は適切なエラーレスポンスを返します。
	*/
}

/*
目的: リモートデバイス上の指定されたパスにあるファイルをリスト表示します。
処理内容:
デバイスに対してファイルリストを取得するリクエストを送信し、応答があれば結果を返します。
タイムアウト時には504 Gateway Timeoutを返します。
*/
// ListDeviceFiles will list files on remote client
//リモートデバイス内の特定のパスにあるファイルやディレクトリのリストを取得するためのAPIエンドポイントを実装しています。以下に詳細な解説を示します。
func ListDeviceFiles(ctx *gin.Context) {
	//form 構造体:
	// クライアントリクエストから path パラメータを受け取る。
	// binding:"required" によって、パスが必須であることを指定。
	var form struct {
		Path string `json:"path" yaml:"path" form:"path" binding:"required"`
	}
	//CheckForm 関数:
	// リクエスト内の必須フィールド（path）が正しく指定されているか検証。
	// デバイスの識別情報も同時にチェックし、成功した場合はターゲットデバイスのUUIDを返す。
	target, ok := utility.CheckForm(ctx, &form)
	//失敗時の処理:
	// 必須フィールドが欠如またはデバイスが見つからない場合、関数はエラーレスポンスを返して終了。
	if !ok {
		return
	}
	//デバイスへのリクエスト送信
	//trigger:
	// ユニークなイベントIDを生成。リクエストとレスポンスを紐づけるために使用。
	trigger := utils.GetStrUUID()
	//SendPackByUUID:
	// FILES_LIST アクションを指定して、ターゲットデバイスにリクエストを送信。
	// 送信内容:
	// Act: リスト取得アクション (FILES_LIST)。
	// Data: ファイルリストを取得したいパス。
	// Event: トリガー識別子。
	common.SendPackByUUID(modules.Packet{Act: `FILES_LIST`, Data: gin.H{`path`: form.Path}, Event: trigger}, target)
	//イベントリスナーの登録
	//AddEventOnce:
	// ターゲットデバイスからのレスポンスを一度だけ処理するためのリスナーを登録。
	// トリガーID (trigger) に基づいてレスポンスを識別。
	// タイムアウトは5秒。
	ok = common.AddEventOnce(func(p modules.Packet, _ *melody.Session) {
		// レスポンスの処理:
		// エラー (p.Code != 0):
		// エラーメッセージを記録し、クライアントに 500 Internal Server Error を返す。
		// 成功 (p.Code == 0):
		// レスポンスデータ (p.Data) をクライアントに 200 OK とともに返す。
		if p.Code != 0 {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		} else {
			ctx.JSON(http.StatusOK, modules.Packet{Code: 0, Data: p.Data})
		}
	}, target, trigger, 5*time.Second)

	//タイムアウト処理
	//イベントリスナーが登録されなかった場合、またはデバイスが応答しない場合:
	// 504 Gateway Timeout を返し、クライアントに応答が遅延したことを通知。
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
	}

	/*
		全体の処理フロー
		リクエスト検証:
			必須フィールド (path) とターゲットデバイスの識別情報をチェック。
		リクエスト送信:
			ターゲットデバイスに対して、指定されたパスのファイルリストを要求。
		レスポンス処理:
			デバイスからの応答を処理し、エラーまたは成功に応じて適切なレスポンスをクライアントに返す。
		タイムアウト対応:
			デバイスが応答しない場合、クライアントにタイムアウトエラーを通知。


		特徴と注意点
		非同期レスポンス:
			イベント駆動の設計により、非同期でレスポンスを処理。
		タイムアウト処理:
			デバイスがオフライン、または応答が遅い場合に対応。
		エラーハンドリング:
			必須フィールドの欠如やデバイス接続の問題を適切に検出し、エラーレスポンスを返す。
		スケーラビリティ:
			同時に複数のデバイスからファイルリストを取得するようなシナリオにも対応可能。
	*/
}

/*
目的: リモートデバイスからファイルを取得し、ブラウザに提供します。
処理内容:
デバイスに対してファイル取得リクエストを送信し、応答があればファイルデータを転送します。
部分的なファイル取得も可能で、HTTPのRangeヘッダーに対応しています。
タイムアウト時には504 Gateway Timeoutを返します。
*/
// GetDeviceFiles will try to get send a packet to
// client and let it upload the file specified.
//クライアントからのリクエストに基づいてデバイスからファイルを取得し、
//HTTPレスポンスとしてクライアントに送信するAPIエンドポイントを実装しています。
//また、部分的なデータ取得（Rangeヘッダー）やプレビュー機能にも対応しています。
func GetDeviceFiles(ctx *gin.Context) {
	//入力データのバインディングと検証
	//Files:
	// ファイルパスのリストを受け取ります（必須）。
	//Preview:
	// プレビューフラグ。ファイルをダウンロードせずに簡易情報を表示するかどうかを示します。
	var form struct {
		Files   []string `json:"files" yaml:"files" form:"files" binding:"required"`
		Preview bool     `json:"preview" yaml:"preview" form:"preview"`
	}
	//CheckForm:
	// リクエストが正しい形式か検証し、ターゲットデバイスを取得します。
	target, ok := utility.CheckForm(ctx, &form)
	if !ok {
		return
	}
	//検証エラー:
	// 必須フィールドが不足している場合は、400 Bad Request を返します。
	if len(form.Files) == 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	//ファイル取得リクエストの準備
	//bridgeID と trigger:
	// ユニークなIDを生成。ブリッジ（データ転送）とレスポンスの識別に使用します。
	bridgeID := utils.GetStrUUID()
	trigger := utils.GetStrUUID()
	//rangeStart, rangeEnd:
	// 部分的なデータ取得（Range ヘッダー）に対応するための開始位置と終了位置。
	var rangeStart, rangeEnd int64
	var err error
	//partial:
	// 部分取得がリクエストされたかどうかを示すフラグ。
	partial := false
	{
		command := gin.H{`files`: form.Files, `bridge`: bridgeID}
		//Rangeヘッダーの処理
		rangeHeader := ctx.GetHeader(`Range`)
		//Range ヘッダー:
		// データの部分的な取得を要求するHTTPヘッダー。
		// bytes=start-end の形式で指定。
		if len(rangeHeader) > 6 {
			if rangeHeader[:6] != `bytes=` {
				ctx.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
				return
			}

			// 検証:
			// ヘッダーが正しい形式かチェック。
			// 範囲が無効な場合（例: end < start）、416 Range Not Satisfiable を返します。
			rangeHeader = strings.TrimSpace(rangeHeader[6:])
			rangesList := strings.Split(rangeHeader, `,`)
			if len(rangesList) > 1 {
				ctx.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			// start
			r := strings.Split(rangesList[0], `-`)
			rangeStart, err = strconv.ParseInt(r[0], 10, 64)
			if err != nil {
				ctx.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			// end
			if len(r[1]) > 0 {
				rangeEnd, err = strconv.ParseInt(r[1], 10, 64)
				if err != nil {
					ctx.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				if rangeEnd < rangeStart {
					ctx.AbortWithStatus(http.StatusRequestedRangeNotSatisfiable)
					return
				}
				command[`end`] = rangeEnd
			}
			//範囲が指定されている場合、start と end をコマンドに追加。
			command[`start`] = rangeStart
			partial = true
		}
		//デバイスへのリクエスト送信
		//デバイスに対してファイル取得コマンドを送信。
		// Act: FILES_UPLOAD。
		// Data: ファイル情報や範囲情報。
		// Event: トリガー識別子。
		common.SendPackByUUID(modules.Packet{Act: `FILES_UPLOAD`, Data: command, Event: trigger}, target)
	}

	//イベントリスナーの登録
	//デバイスからの応答を非同期で処理。
	// 応答が失敗した場合、エラーメッセージをクライアントに返し、500 Internal Server Error を送信。
	wait := make(chan bool)
	called := false
	common.AddEvent(func(p modules.Packet, _ *melody.Session) {
		called = true
		bridge.RemoveBridge(bridgeID)
		common.RemoveEvent(trigger)
		common.Warn(ctx, `READ_FILES`, `fail`, p.Msg, map[string]any{
			`files`: form.Files,
		})
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		wait <- false
	}, target, trigger)

	//データ転送の設定
	instance := bridge.AddBridgeWithDst(nil, bridgeID, ctx)
	//OnPush:
	// データ転送が開始されたときにヘッダーを設定。
	instance.OnPush = func(bridge *bridge.Bridge) {
		called = true
		common.RemoveEvent(trigger)
		src := bridge.Src
		for k, v := range src.Request.Header {
			if strings.HasPrefix(k, `File`) {
				ctx.Header(k, v[0])
			}
		}

		//ヘッダー設定:
		// ファイル名、サイズ、転送形式（バイナリ/部分取得）を設定。
		if !form.Preview {
			if len(form.Files) == 1 {
				ctx.Header(`Accept-Ranges`, `bytes`)
				if src.Request.ContentLength > 0 {
					ctx.Header(`Content-Length`, strconv.FormatInt(src.Request.ContentLength, 10))
				}
			} else {
				ctx.Header(`Accept-Ranges`, `none`)
			}
			ctx.Header(`Content-Transfer-Encoding`, `binary`)
			ctx.Header(`Content-Type`, `application/octet-stream`)
			filename := src.GetHeader(`FileName`)
			if len(filename) == 0 {
				if len(form.Files) > 1 {
					filename = `Archive.zip`
				} else {
					filename = path.Base(strings.ReplaceAll(form.Files[0], `\`, `/`))
				}
			}
			// ファイルメタ情報を設定
			ctx.Header(`Content-Disposition`, fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename, url.PathEscape(filename)))
		}

		if partial {
			if rangeEnd == 0 {
				rangeEnd, err = strconv.ParseInt(src.GetHeader(`FileSize`), 10, 64)
				if err == nil {
					// 範囲リクエストに対応
					ctx.Header(`Content-Range`, fmt.Sprintf(`bytes %d-%d/%d`, rangeStart, rangeEnd-1, rangeEnd))
				}
			} else {
				// 範囲リクエストに対応
				ctx.Header(`Content-Range`, fmt.Sprintf(`bytes %d-%d/%v`, rangeStart, rangeEnd, src.GetHeader(`FileSize`)))
			}
			ctx.Status(http.StatusPartialContent)
		} else {
			ctx.Status(http.StatusOK)
		}
	}
	//OnFinish:
	// データ転送が完了した場合にログを記録。
	instance.OnFinish = func(bridge *bridge.Bridge) {
		if called {
			common.Info(ctx, `READ_FILES`, `success`, ``, map[string]any{
				`files`: form.Files,
			})
		}
		wait <- false
	}

	//タイムアウト処理

	// 応答があった場合:
	// イベントの終了を待機。
	select {
	case <-wait:

		//デバイスが応答しない場合:
	//タイムアウト（5秒）後にエラーを返す。
	case <-time.After(5 * time.Second):
		if !called {
			bridge.RemoveBridge(bridgeID)
			common.RemoveEvent(trigger)
			common.Warn(ctx, `READ_FILES`, `fail`, `timeout`, map[string]any{
				`files`: form.Files,
			})
			ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
		} else {
			<-wait
		}
	}
	close(wait)

	/*
		全体の処理フロー
		リクエストの検証:
			必要なパラメータ（Files や Range）を検証。
			デバイスへのリクエスト送信:
			対象デバイスにファイル取得コマンドを送信。

		データ転送:
			デバイスから送信されたデータをクライアントにストリーミング。

			タイムアウト対応:
			デバイスが応答しない場合、エラーメッセージを返す。
	*/
}

/*
目的: リモートデバイスからテキストファイルを取得し、ブラウザに提供します。
処理内容:
ファイル取得のためのリクエストをデバイスに送り、ファイルデータを取得します。
タイムアウト時には504 Gateway Timeoutを返します。
*/
// GetDeviceTextFile will try to get send a packet to
// client and let it upload the text file.
//リモートデバイスからテキストファイルを取得してクライアントに提供するAPIエンドポイントを実現するものです。以下で、コードの処理内容を日本語で詳細に説明します。
func GetDeviceTextFile(ctx *gin.Context) {
	//入力のバリデーション
	//クライアントから送られてきた file パラメータが正しいかチェックします。
	var form struct {
		File string `json:"file" yaml:"file" form:"file" binding:"required"`
	}
	// CheckForm 関数を使ってターゲットデバイスが正しく設定されているか確認します。
	target, ok := utility.CheckForm(ctx, &form)
	if !ok {
		return
	}
	// file が空の場合、HTTP 400 (Bad Request) エラーを返します。
	if len(form.File) == 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}

	//デバイスへのコマンド送信
	bridgeID := utils.GetStrUUID()
	trigger := utils.GetStrUUID()
	//bridgeID と trigger を生成して、一意のリクエストを識別します。
	// FILE_UPLOAD_TEXT コマンドをリモートデバイスに送信します。
	// ファイル名 (form.File) と bridgeID を含むパケットをデバイスに送ります。
	common.SendPackByUUID(modules.Packet{Act: `FILE_UPLOAD_TEXT`, Data: gin.H{
		`file`:   form.File,
		`bridge`: bridgeID,
	}, Event: trigger}, target)

	//イベントリスナーの設定
	wait := make(chan bool)
	called := false

	// デバイスからのレスポンスを待ち受けるイベントリスナーを登録します。
	common.AddEvent(func(p modules.Packet, _ *melody.Session) {
		called = true
		bridge.RemoveBridge(bridgeID)
		common.RemoveEvent(trigger)
		//デバイスがエラーを返した場合、ブリッジを削除し、HTTP 500 (Internal Server Error) を返します。
		// リクエストが正常に処理された場合、このリスナーは後続の処理で役割を果たします。
		common.Warn(ctx, `READ_TEXT_FILE`, `fail`, p.Msg, map[string]any{
			`file`: form.File,
		})
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		wait <- false
	}, target, trigger)

	//ブリッジの初期化
	//ブリッジとは？:
	// ブリッジは、リモートデバイスからのデータをクライアントにストリーム形式で転送する仕組みです。
	instance := bridge.AddBridgeWithDst(nil, bridgeID, ctx)

	//OnPush コールバック:
	// デバイスがファイルを送信し始めた際に呼び出されます。
	instance.OnPush = func(bridge *bridge.Bridge) {
		called = true
		common.RemoveEvent(trigger)
		src := bridge.Src
		for k, v := range src.Request.Header {
			if strings.HasPrefix(k, `File`) {
				ctx.Header(k, v[0])
			}
		}
		// ヘッダーを設定し、レスポンスとしてファイルをクライアントに送信します。
		ctx.Header(`Accept-Ranges`, `none`)
		ctx.Header(`Content-Transfer-Encoding`, `binary`)
		// ファイル名やデータ形式 (application/octet-stream) を適切に設定します。
		ctx.Header(`Content-Type`, `application/octet-stream`)
		filename := src.GetHeader(`FileName`)
		if len(filename) == 0 {
			filename = path.Base(strings.ReplaceAll(form.File, `\`, `/`))
		}
		ctx.Header(`Content-Disposition`, fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, filename, url.PathEscape(filename)))
		ctx.Status(http.StatusOK)
	}

	instance.OnFinish = func(bridge *bridge.Bridge) {
		if called {
			common.Info(ctx, `READ_TEXT_FILE`, `success`, ``, map[string]any{
				`file`: form.File,
			})
		}
		wait <- false
	}

	//タイムアウト処理
	//タイムアウトが発生しないように、最大5秒間レスポンスを待ちます。
	select {
	//デバイスが応答を返し、ファイル送信が開始されると、処理が正常終了します。
	case <-wait:

		//5秒以内に応答がない場合、ブリッジを削除し、HTTP 504 (Gateway Timeout) エラーをクライアントに返します。
	case <-time.After(5 * time.Second):
		if !called {
			bridge.RemoveBridge(bridgeID)
			common.RemoveEvent(trigger)
			common.Warn(ctx, `READ_TEXT_FILE`, `fail`, `timeout`, map[string]any{
				`file`: form.File,
			})
			ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
		} else {
			<-wait
		}
	}
	close(wait)

	/*
		全体の処理フロー
		入力の確認:
			クライアントから送信されたファイル名とターゲットデバイス情報を検証します。
		コマンド送信:
			デバイスにファイル送信を指示するコマンドを送信します。
		レスポンスの処理:
			デバイスからの応答を待ち、ファイルが送信されるとクライアントにストリーミング形式でデータを転送します。
		タイムアウトの管理:
			応答がない場合、タイムアウト処理を実行し、エラーをクライアントに通知します。

		特徴
		動的ファイル送信:
			デバイスから取得したテキストファイルをリアルタイムでクライアントに転送。
		エラーハンドリング:
			デバイスのエラーや応答遅延を適切に処理。
		ヘッダー管理:
			ファイルのメタデータ (例: 名前、形式) をクライアントに適切に提供。
		非同期イベント管理:
			イベントドリブンなアプローチでデバイスの応答を効率的に処理。
	*/
}

/*
目的: ブラウザからアップロードされたファイルをリモートデバイスに転送します。
処理内容:
ファイルを指定されたパスにアップロードし、完了後にレスポンスを返します。
タイムアウト時には504 Gateway Timeoutを返します。
*/
// UploadToDevice handles file from browser
// and transfer it to device.
//クライアントからリモートデバイスにファイルをアップロードするためのAPIエンドポイントを実装しています。
func UploadToDevice(ctx *gin.Context) {
	//入力データのバリデーション
	//クライアントが送信した path と file の値を確認します。
	var form struct {
		Path string `json:"path" yaml:"path" form:"path" binding:"required"`
		File string `json:"file" yaml:"file" form:"file" binding:"required"`
	}
	// 両方が必須 (binding:"required") であり、空の場合は HTTP 400 (Bad Request) を返します。
	target, ok := utility.CheckForm(ctx, &form)
	if !ok {
		return
	}
	if len(form.File) == 0 || len(form.Path) == 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}

	//アップロード先情報の設定
	//一意のIDを生成 (bridgeID と trigger)。
	// bridgeID: ブリッジを識別するため。
	// trigger: イベントを識別するため。
	bridgeID := utils.GetStrUUID()
	trigger := utils.GetStrUUID()

	wait := make(chan bool)
	called := false
	response := false

	//アップロード先パス (fileDest) を作成します。
	// 例: form.Path = /home/user と form.File = test.txt の場合、fileDest = /home/user/test.txt。
	fileDest := path.Join(form.Path, form.File)
	fileSize := ctx.Request.ContentLength

	//ブリッジの作成
	//イベントリスナーを登録
	common.AddEvent(func(p modules.Packet, _ *melody.Session) {
		called = true
		response = true
		bridge.RemoveBridge(bridgeID)
		common.RemoveEvent(trigger)
		//リモートデバイスからエラーが返された場合、エラー内容をクライアントに返します。
		//エラー時:
		// ブリッジ (bridgeID) を削除。
		// イベント (trigger) を削除。
		// HTTP 500 (Internal Server Error) をクライアントに返す。
		common.Warn(ctx, `UPLOAD_FILE`, `fail`, p.Msg, map[string]any{
			`dest`: fileDest,
			`size`: fileSize,
		})
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		wait <- false
	}, target, trigger)

	//アップロード処理の開始
	//ブリッジの初期化:
	// AddBridgeWithSrc: クライアントからデバイスにデータを送信するためのブリッジを作成。
	instance := bridge.AddBridgeWithSrc(nil, bridgeID, ctx)

	//OnPull コールバック:
	// リモートデバイスがデータを受信する準備ができた場合に呼び出される。
	// 必要なHTTPヘッダー (Content-Type など) を設定。
	instance.OnPull = func(bridge *bridge.Bridge) {
		called = true
		common.RemoveEvent(trigger)
		dst := bridge.Dst
		if ctx.Request.ContentLength > 0 {
			dst.Header(`Content-Length`, strconv.FormatInt(ctx.Request.ContentLength, 10))
		}
		dst.Header(`Accept-Ranges`, `none`)
		dst.Header(`Content-Transfer-Encoding`, `binary`)
		dst.Header(`Content-Type`, `application/octet-stream`)
		dst.Header(`Content-Disposition`, fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, form.File, url.PathEscape(form.File)))
	}

	//タイムアウトと完了時の処理
	//OnFinish コールバック:
	// アップロードが完了した際に呼び出される。
	// ログを記録し、完了通知を送信。
	instance.OnFinish = func(bridge *bridge.Bridge) {
		if called {
			common.Info(ctx, `UPLOAD_FILE`, `success`, ``, map[string]any{
				`dest`: fileDest,
				`size`: fileSize,
			})
		}
		wait <- false
	}
	common.SendPackByUUID(modules.Packet{Act: `FILES_FETCH`, Data: gin.H{
		`path`:   form.Path,
		`file`:   form.File,
		`bridge`: bridgeID,
	}, Event: trigger}, target)

	//タイムアウト管理
	select {
	//ファイルが正常にアップロードされた場合、HTTP 200 (OK) を返す。
	case <-wait:
		if !response {
			ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
		}

		//タイムアウト (5秒) の場合、HTTP 504 (Gateway Timeout) を返す。
		// デバイスからエラーが返された場合は、その内容を通知。
	case <-time.After(5 * time.Second):
		if !called {
			bridge.RemoveBridge(bridgeID)
			common.RemoveEvent(trigger)
			if !response {
				common.Warn(ctx, `UPLOAD_FILE`, `fail`, `timeout`, map[string]any{
					`dest`: fileDest,
					`size`: fileSize,
				})
				ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
			}
		} else {
			<-wait
			if !response {
				ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
			}
		}
	}
	close(wait)

	/*
		特徴
		入力の厳密な検証:
			必須パラメータ (path, file) をチェック。
		非同期通信:
			イベントリスナーとブリッジを使用して、非同期にアップロード処理を管理。
		タイムアウト管理:
			適切なレスポンスが得られない場合、タイムアウトを通知。
		動的HTTPヘッダー:
			ファイルサイズや名前を基にヘッダーを設定し、柔軟に対応。

	*/
}
