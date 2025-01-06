package utility

import (
	"Spark/modules"
	"Spark/server/common"
	"Spark/server/config"
	"Spark/utils"
	"Spark/utils/melody"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

/*
WebSocket通信やデバイス管理、およびクライアントとサーバー間のコマンド実行に関する一連のユーティリティ機能を提供しています。
以下に、このコードの主要部分を解説します。


リモートデバイス管理を行うWebサーバーの一部として機能します。リモートデバイスとブラウザとの通信をWebSocketで実現し、
クライアントデバイスの管理やコマンド実行、デバイスのヘルスチェックを行うための重要なユーティリティ関数が含まれています。
また、暗号化されたデータ通信やバージョン管理などもサポートしており、実用的なデバイス管理ソリューションを提供します。
*/

// 送信関数の型
type Sender func(pack modules.Packet, session *melody.Session) bool

/*
説明: リクエストから接続UUIDまたはデバイスIDを取得して、フォームデータが有効かどうかを確認します。
機能:
form 引数が nil でない場合、リクエストのデータをバインド（デシリアライズ）し、不正なデータがあれば400エラーを返します。
UUIDまたはデバイスIDをチェックし、デバイスが存在しない場合は502エラーを返します。
デバイスが存在する場合は、UUIDを返します。
*/
// CheckForm checks if the form contains the required fields.
// Every request must contain connection UUID or device ID.
//Ginフレームワークを用いたWeb APIの一部で、リクエストから送られるフォームデータを検証し、
//リモートデバイスまたは接続UUIDが有効であるかを確認するための処理を行っています。
/*
CheckForm は、指定されたフォームデータを検証し、リモートデバイス（Device）または接続UUID（Conn）のいずれかが正しいかどうかを確認します。
データが正しくない場合、エラーレスポンスを返して処理を終了します。
データが有効であれば、接続UUIDを返します。

ctx: Gin のコンテキストオブジェクト。リクエストやレスポンスを操作するために使用します。
form: 任意の構造体（any 型）。リクエストデータをマッピングするために使用されます。
*/
func CheckForm(ctx *gin.Context, form any) (string, bool) {
	//フォームデータの検証
	var base struct {
		Conn   string `json:"uuid" yaml:"uuid" form:"uuid"`
		Device string `json:"device" yaml:"device" form:"device"`
	}
	//フォームデータのバインド:
	//form が指定されている場合、ctx.ShouldBind(form) を使用してリクエストデータを form にマッピングします。
	if form != nil && ctx.ShouldBind(form) != nil {
		//バインドが失敗した場合、400 Bad Request とともにエラーメッセージを返し、処理を終了します。
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return ``, false
	}
	//基本データの検証
	//base 構造体のバインド
	//uuid (接続UUID) と device (デバイスID) を含む base 構造体にリクエストデータをマッピングします。
	if ctx.ShouldBind(&base) != nil || (len(base.Conn) == 0 && len(base.Device) == 0) {
		//失敗条件:
		// バインドに失敗した場合。
		// Conn および Device の両方が空の場合。
		//400 Bad Request を返し、処理を終了します。
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return ``, false
	}

	//デバイスまたは接続UUIDの確認
	//CheckDevice を使用した確認
	/*
		common.CheckDevice 関数を呼び出し、指定された Device または Conn が有効であるかを確認します。
		有効な場合:
		接続UUID (connUUID) を返します。
		無効な場合:
		502 Bad Gateway を返し、処理を終了します。
	*/
	connUUID, ok := common.CheckDevice(base.Device, base.Conn)
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, modules.Packet{Code: 1, Msg: `${i18n|COMMON.DEVICE_NOT_EXIST}`})
		return ``, false
	}
	//接続UUIDのコンテキストへの追加
	/*
		接続UUIDの保存:
		リクエストのコンテキストに接続UUIDを追加します。この後の処理で使用可能になります。
		戻り値:
		接続UUID (connUUID) と成功フラグ (true) を返します。
	*/
	ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), `ConnUUID`, connUUID))
	return connUUID, true

	/*
		関数の実行フロー
		フォームデータの検証:
			指定されたフォーム構造体がリクエストデータにマッピングできるかを確認します。

		基本データの検証:
			uuid または device がリクエストに含まれていることを確認します。
			デバイスまたは接続UUIDの確認:
			CheckDevice を使用して、指定されたデバイスまたはUUIDが存在するかを検証します。

		結果の返却:
			検証が成功した場合、接続UUIDを返します。
			失敗した場合、適切なエラーレスポンスを返して終了します。

		関数の用途
			他のAPIエンドポイントで、リクエストデータの検証と接続UUIDの取得に使用されることを想定しています。
			検証の一貫性を保つための共通コードとして設計されています。
	*/
}

/*
説明: デバイス情報に関するイベント（接続ハンドシェイクやデバイス情報の更新）を処理します。
機能:
クライアントから送信されたデータをデシリアライズして、デバイスの情報を更新します。
新しいデバイスが接続された場合、同じデバイスIDを持つ既存のセッションがあれば、それを終了させます。
デバイスのCPU、RAM、ネットワークなどの情報を更新し、デバイスがオンラインであることをログに記録します。
*/
// OnDevicePack handles events about device info.
// Such as websocket handshake and update device info.
/*
OnDevicePack は、デバイス（クライアント）から送られてきたデータパケットを処理し、デバイス情報を更新または登録します。
また、すでに同一のデバイスが接続している場合、そのセッションを強制的に終了します。
以下に、詳細な解説を行います。
*/
//data []byte: デバイスから受信したパケットデータ（JSON形式）。
//session *melody.Session: デバイスとの現在のセッション。
func OnDevicePack(data []byte, session *melody.Session) error {
	//パケットデータの解析
	var pack struct {
		Code   int            `json:"code,omitempty"`
		Act    string         `json:"act,omitempty"`
		Msg    string         `json:"msg,omitempty"`
		Device modules.Device `json:"data"`
	}
	//受信したデータを pack 構造体にデシリアライズします。
	err := utils.JSON.Unmarshal(data, &pack)
	//JSON解析に失敗した場合、セッションを閉じてエラーを返します。
	if err != nil {
		session.Close()
		return err
	}

	//クライアントのWANアドレスを設定
	addr, ok := session.Get(`Address`)

	//セッションに保存されているクライアントのWANアドレスを取得し、デバイス情報に設定します。
	//アドレスが取得できない場合は、"Unknown" を設定します
	if ok {
		pack.Device.WAN = addr.(string)
	} else {
		pack.Device.WAN = `Unknown`
	}

	//DEVICE_UP アクションの処理
	//デバイスが初回接続した場合の処理。
	//デバイスがすでに接続している場合、その既存セッションを閉じ、新しい接続を優先します。
	if pack.Act == `DEVICE_UP` {
		// Check if this device has already connected.
		// If so, then find the session and let client quit.
		// This will keep only one connection remained per device.
		exSession := ``

		//common.Devices.IterCb を使用して、現在接続中のデバイスを走査します
		common.Devices.IterCb(func(uuid string, device *modules.Device) bool {
			// デバイスが一致した場合
			if device.ID == pack.Device.ID {
				exSession = uuid
				target, ok := common.Melody.GetSessionByUUID(uuid)
				//同じ device.ID を持つデバイスが見つかった場合、そのセッションを取得し、OFFLINE メッセージを送信してセッションを閉じます。
				if ok {
					common.SendPack(modules.Packet{Act: `OFFLINE`}, target)
					target.Close()
				}
				return false
			}
			return true
		})
		//古いセッションを common.Devices から削除します。
		if len(exSession) > 0 {
			common.Devices.Remove(exSession)
		}
		//新しいセッションを common.Devices に登録します。
		common.Devices.Set(session.UUID, &pack.Device)

		//新しい接続が成功した場合、CLIENT_ONLINE ログを記録します。
		common.Info(nil, `CLIENT_ONLINE`, ``, ``, map[string]any{
			`device`: map[string]any{
				`name`: pack.Device.Hostname,
				`ip`:   pack.Device.WAN,
			},
		})
	} else {
		//既存デバイス情報の更新
		//デバイスが既存のセッションで登録されている場合、その情報を更新します。
		device, ok := common.Devices.Get(session.UUID)
		/*
			更新するフィールド:
			CPU: CPU使用率。
			RAM: メモリ使用量。
			Net: ネットワーク使用状況。
			Disk: ディスク使用量。
			Uptime: 起動時間。
		*/
		if ok {
			device.CPU = pack.Device.CPU
			device.RAM = pack.Device.RAM
			device.Net = pack.Device.Net
			device.Disk = pack.Device.Disk
			device.Uptime = pack.Device.Uptime
		}
	}
	//デバイスへのレスポンス送信
	common.SendPack(modules.Packet{Code: 0}, session)
	return nil

	/*
		関数のフロー
		データの解析:
		受信したデータを解析して pack にデシリアライズ。
		WANアドレスの設定:
		クライアントのWANアドレスをデバイス情報に設定。
		DEVICE_UP の処理:
		同じデバイスIDの既存セッションがある場合、そのセッションを閉じる。
		新しいセッションを登録。
		既存デバイスの更新:
		新しい情報でデバイスのステータスを更新。
		レスポンス送信:
		処理結果をデバイスに通知。

		デバイスからの接続要求とデータ更新を処理する役割を果たします。同一デバイスの複数接続を排除し、常に最新の接続情報を維持することで、デバイス管理の一貫性を確保します。
	*/

	/*
		初回接続なのに、すでにデバイスが存在するケース
			1. 冗長な接続が発生する可能性
			デバイスが何らかの理由で同一の device.ID を持つ複数の接続を行う可能性があります。具体的には：

			不安定なネットワーク環境: デバイスが接続切断を検出できず、新しいセッションを開始してしまう。
			クライアントの再起動や再接続: デバイスが意図せず複数回接続を試みる。
			競合したプロセス: デバイス上で同じクライアントプログラムが複数起動してしまう。
			これにより、1つのデバイスに対して複数のセッションが確立されてしまうと、データのやり取りに混乱が生じるため、既存セッションを検出して優先的に終了させる必要があります。

			2. デバイスIDの再利用
			固定ID: デバイスが固定のID（device.ID）を使用する設計の場合、古い接続が残っている状態で新しい接続が発生する可能性があります。
			例: デバイスが電源を切らずに再接続した。
			この場合、システムは古い接続を明示的に切断し、新しい接続を優先することで、最新の接続を確実に有効化します。

			3. デバイスが適切に切断されなかった場合
			デバイス側が接続終了メッセージ（DEVICE_DOWN）を送信する前に接続が切れてしまう場合があります。
			例: ネットワークの突然の切断や、プロセスの強制終了。
			この場合、サーバー側では古い接続が「存在する」と認識され続けますが、実際には利用不能です。
			この状況を検出し、新しい接続が正常に動作するようにするために、古い接続を終了します。

			4. セキュリティの考慮
			同一の device.ID を持つデバイスが複数接続している場合、不正アクセスや異常な動作の兆候とみなすことができます。
			例: クライアントがクローンされ、複数の拠点から同時に接続を試みている。
			サーバーはこの状況を検出し、古い接続を閉じて新しい接続を優先することで、不正な競合を最小限に抑えます。

			5. 運用の一貫性を保つ
			システム管理のシンプルさ:
			同じデバイスが複数のセッションを持つと、管理者にとってどのセッションが「正しい」ものかが不明確になります。
			常に「最新の接続」を維持するポリシーを取ることで、サーバー側と管理者の両方にとって一貫性が保たれます。

	*/
}

/*
説明: クライアントが最新バージョンであるかどうかを確認し、必要に応じて更新を提供します。
機能:
クライアントからのOS、アーキテクチャ、コミット情報を取得し、サーバー上の現在のバージョンと比較します。
クライアントが最新でない場合、クライアントに更新データを提供します（client.cfgなどの構成データを含むバイナリファイルの形で）。
*/
// CheckUpdate will check if client need update and return latest client if so.
//クライアントがサーバーから更新をリクエストする際の処理を行うエンドポイント CheckUpdate の実装です。
//クライアントのOS、アーキテクチャ、コミット情報を基に、更新が必要か確認し、更新データを送信します。
func CheckUpdate(ctx *gin.Context) {
	//フォームデータのバインドとバリデーション
	var form struct {
		OS     string `form:"os" binding:"required"`
		Arch   string `form:"arch" binding:"required"`
		Commit string `form:"commit" binding:"required"`
	}

	//クライアントから送信されたリクエストパラメータ (os, arch, commit) を form 構造体にバインド。
	if err := ctx.ShouldBind(&form); err != nil {
		//必須項目が欠けている場合、HTTPステータス 400 Bad Request を返して終了。
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}

	//コミットの一致確認
	//クライアントが送信した Commit がサーバーの現在の config.COMMIT と一致する場合、更新不要と判断し、HTTPステータス 200 OK を返します
	if form.Commit == config.COMMIT {
		ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
		common.Warn(ctx, `CLIENT_UPDATE`, `success`, `latest`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}

	//クライアント用ビルドファイルの検証
	tpl, err := os.Open(fmt.Sprintf(config.BuiltPath, form.OS, form.Arch))
	//指定されたOSとアーキテクチャに対応するビルド済みファイル（テンプレート）が存在するか確認。
	if err != nil {
		//存在しない場合、404 Not Found を返して終了。
		ctx.AbortWithStatusJSON(http.StatusNotFound, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.NO_PREBUILT_FOUND}`})
		common.Warn(ctx, `CLIENT_UPDATE`, `fail`, `no prebuild asset`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}
	defer tpl.Close()

	//リクエストのボディサイズ検証
	const MaxBodySize = 384 // This is size of client config buffer.

	//クライアントから送信された設定データ（リクエストボディ）が許容サイズ（384バイト）を超えていないか検証。
	if ctx.Request.ContentLength > MaxBodySize {
		//超えている場合、413 Payload Too Large を返して終了
		ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, modules.Packet{Code: 1})
		common.Warn(ctx, `CLIENT_UPDATE`, `fail`, `config too large`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}

	//bodyデータの取得
	body, err := ctx.GetRawData()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: 1})
		common.Warn(ctx, `CLIENT_UPDATE`, `fail`, `read config fail`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}

	//クライアントの認証
	//CheckClientReq を用いてクライアントが有効なリクエストか確認。
	session := common.CheckClientReq(ctx)
	if session == nil {
		//認証失敗時は 401 Unauthorized を返して終了。
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, modules.Packet{Code: 1})
		common.Warn(ctx, `CLIENT_UPDATE`, `fail`, `check config fail`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}

	common.Info(ctx, `CLIENT_UPDATE`, `success`, `updating`, map[string]any{
		`client`: map[string]any{
			`os`:     form.OS,
			`arch`:   form.Arch,
			`commit`: form.Commit,
		},
		`server`: config.COMMIT,
	})

	//更新データ送信
	//HTTPヘッダーの設定
	//サーバーのコミットバージョンやデータ形式、サイズをクライアントに通知。
	ctx.Header(`Spark-Commit`, config.COMMIT)
	ctx.Header(`Accept-Ranges`, `none`)
	ctx.Header(`Content-Transfer-Encoding`, `binary`)
	ctx.Header(`Content-Type`, `application/octet-stream`)
	if stat, err := tpl.Stat(); err == nil {
		ctx.Header(`Content-Length`, strconv.FormatInt(stat.Size(), 10))
	}

	//プレースホルダーの置換と送信
	cfgBuffer := bytes.Repeat([]byte{'\x19'}, 384)
	prevBuffer := make([]byte, 0)

	//テンプレートファイルから読み込んだデータ（バイト列）を逐次クライアントに送信。
	for {
		thisBuffer := make([]byte, 1024)
		n, err := tpl.Read(thisBuffer)
		thisBuffer = thisBuffer[:n]
		tempBuffer := append(prevBuffer, thisBuffer...)
		bufIndex := bytes.Index(tempBuffer, cfgBuffer)
		//バッファ内に特定のプレースホルダー（cfgBuffer）が見つかった場合、クライアントから送信された設定データ（body）に置換。
		if bufIndex > -1 {
			tempBuffer = bytes.Replace(tempBuffer, cfgBuffer, body, -1)
		}

		//送信
		ctx.Writer.Write(tempBuffer[:len(prevBuffer)])
		prevBuffer = tempBuffer[len(prevBuffer):]
		if err != nil {
			break
		}
	}

	//最後に残ったデータを送信。
	if len(prevBuffer) > 0 {
		ctx.Writer.Write(prevBuffer)
		prevBuffer = []byte{}
	}

	/*
		全体の処理フロー
		リクエストのバリデーション: クライアントから送信されたパラメータやボディサイズ、認証情報を検証。
		更新不要の場合の処理: コミットが一致する場合は更新不要と判断し終了。
		テンプレートファイルの取得: 指定されたOSとアーキテクチャに対応するファイルを開く。
		データ置換と送信: ファイルを逐次クライアントに送信し、特定のバッファを設定データに置換。


		このコードの特徴
		効率的なストリーミング送信: 大きなファイルを一度に読み込むのではなく、バッファ単位で処理。
		プレースホルダー置換: クライアント固有の設定をファイルに埋め込んでカスタマイズ可能。
		セキュリティの考慮: クライアント認証とサイズ制限で不正なデータ送信を防止。
		拡張性: OSやアーキテクチャの種類に応じて柔軟に対応。
	*/
}

/*
説明: 指定されたコマンドをリモートデバイス上で実行します。
機能:
コマンドと引数をリクエストから取得し、ターゲットデバイスに対してコマンドを送信します。
5秒以内にレスポンスが返ってこない場合、タイムアウトエラーを返します。
*/
// ExecDeviceCmd execute command on device.
//クライアントデバイス上でコマンドを実行するためのエンドポイント ExecDeviceCmd を実装しています。クライアントがリクエストを送信すると、サーバーが適切なデバイスにコマンドを送信し、その実行結果を処理します。
func ExecDeviceCmd(ctx *gin.Context) {
	//リクエストパラメータのバリデーション
	/*
		form 構造体:
		Cmd: 実行するコマンド（必須）。
		Args: コマンドの引数（オプション）。
	*/
	var form struct {
		Cmd  string `json:"cmd" yaml:"cmd" form:"cmd" binding:"required"`
		Args string `json:"args" yaml:"args" form:"args"`
	}
	//CheckForm を使用して、リクエストパラメータが正しい形式であるかを確認し、ターゲットデバイス（target）を特定。
	target, ok := CheckForm(ctx, &form)
	//検証に失敗した場合、適切なエラーレスポンスを返して終了。
	if !ok {
		return
	}

	//コマンドのバリデーション
	if len(form.Cmd) == 0 {
		//コマンド (form.Cmd) が空の場合は、400 Bad Request を返して終了。
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	//trigger はユニークな識別子として生成され、リクエストとレスポンスを紐づけるために使用。
	trigger := utils.GetStrUUID()
	//SendPackByUUID を使用して、デバイスにコマンド実行リクエストを送信。
	// Act: アクション名として COMMAND_EXEC を指定。
	// Data: 実行するコマンドとその引数を送信。
	// Event: トリガー識別子。
	common.SendPackByUUID(modules.Packet{Act: `COMMAND_EXEC`, Data: gin.H{`cmd`: form.Cmd, `args`: form.Args}, Event: trigger}, target)

	//イベントリスナーの登録
	//AddEventOnce:
	// トリガーに基づいて、デバイスからのレスポンスを一度だけ処理するリスナーを登録。
	// 5秒間（5*time.Second）レスポンスを待機。
	ok = common.AddEventOnce(func(p modules.Packet, _ *melody.Session) {
		/*
			レスポンスの処理:
			成功 (p.Code == 0) の場合:
			ログに成功情報を記録 (common.Info)。
			クライアントに 200 OK を返す。
			失敗 (p.Code != 0) の場合:
			エラー情報を記録 (common.Warn)。
			クライアントに 500 Internal Server Error を返す。
		*/
		if p.Code != 0 {
			common.Warn(ctx, `EXEC_COMMAND`, `fail`, p.Msg, map[string]any{
				`cmd`:  form.Cmd,
				`args`: form.Args,
			})
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		} else {
			common.Info(ctx, `EXEC_COMMAND`, `success`, ``, map[string]any{
				`cmd`:  form.Cmd,
				`args`: form.Args,
			})
			ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
		}
	}, target, trigger, 5*time.Second)

	//タイムアウト処理
	//5秒以内にデバイスからレスポンスがなかった場合:
	// タイムアウトエラーとしてログを記録。
	// クライアントに 504 Gateway Timeout を返す。
	if !ok {
		common.Warn(ctx, `EXEC_COMMAND`, `fail`, `timeout`, map[string]any{
			`cmd`:  form.Cmd,
			`args`: form.Args,
		})
		ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
	}

	/*
		全体の処理フロー
		リクエストのバリデーション:
		クライアントが送信したコマンドとターゲットデバイスを検証。
		コマンドの送信:
		指定されたデバイスにコマンドを送信。
		レスポンスの待機と処理:
		成功時: クライアントに成功レスポンスを返す。
		失敗時: エラーメッセージとともに適切なHTTPステータスを返す。
		タイムアウト処理:
		指定時間内にレスポンスがない場合、タイムアウトエラーを返す。


		このコードの特徴
		非同期イベント駆動設計:
		サーバーはリクエストを送信し、レスポンスを非同期で待機。
		タイムアウトを設定することで、レスポンス遅延時の処理を明確化。
		エラー処理の明確化:
		リクエストのバリデーション、デバイスの状態確認、レスポンス処理それぞれでエラー時のレスポンスを適切に設定。
		拡張性:
		デバイスに対して汎用的なコマンド実行を提供するため、他のアクションにも応用可能。
	*/
}

/*
説明: 接続されているすべてのクライアントデバイスの情報を取得して返します。
機能:
common.Devices に保存されているすべてのデバイス情報を取得し、HTTPレスポンスとして返します。
*/
// GetDevices will return all info about all clients.
func GetDevices(ctx *gin.Context) {
	devices := map[string]any{}

	// すべてのデバイスを取得
	common.Devices.IterCb(func(uuid string, device *modules.Device) bool {
		devices[uuid] = *device
		return true
	})
	ctx.JSON(http.StatusOK, modules.Packet{Code: 0, Data: devices})
}

/*
説明: 特定のコマンド（ロック、ログオフ、シャットダウンなど）をクライアントデバイスに送信します。
機能:
act パラメータを取得し、それに基づいてリモートデバイスに対してコマンドを実行します。
クライアントがオフラインの場合でも、コマンドが成功したと見なします。
*/
// CallDevice will call client with command from browser.
//デバイスに対して特定の操作をリモートで実行するためのAPIエンドポイント CallDevice を実装しています。
//指定されたアクションをデバイスに送信し、応答を待つ仕組みが構築されています。
func CallDevice(ctx *gin.Context) {

	//アクションの検証
	//リクエストから act パラメータ（アクション）を取得し、大文字に変換。
	act := strings.ToUpper(ctx.Param(`act`))
	//act が空の場合、400 Bad Request エラーを返して終了
	if len(act) == 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}

	//許可されたアクションの確認
	{
		//許可されたアクション（LOCK, LOGOFF, HIBERNATE など）と比較し、有効かどうか確認。
		actions := []string{`LOCK`, `LOGOFF`, `HIBERNATE`, `SUSPEND`, `RESTART`, `SHUTDOWN`, `OFFLINE`}
		ok := false
		for _, v := range actions {
			if v == act {
				ok = true
				break
			}
		}

		//アクションがリストに含まれていない場合、400 Bad Request エラーを返し、ログに警告メッセージを記録。
		if !ok {
			common.Warn(ctx, `CALL_DEVICE`, `fail`, `invalid act`, map[string]any{
				`act`: act,
			})
			ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
			return
		}
	}

	//デバイスの検証
	//デバイスが存在するか、またその接続が有効かを CheckForm 関数で検証。
	// 無効な場合、適切なエラーレスポンスを返して終了。
	connUUID, ok := CheckForm(ctx, nil)
	if !ok {
		return
	}

	//アクションの送信
	//trigger: ユニークなトリガー識別子を生成。サーバーとクライアント間でリクエストとレスポンスを紐づけるために使用。
	trigger := utils.GetStrUUID()

	//SendPackByUUID: デバイスに対して指定されたアクションを送信。
	// Act: 実行するアクション（例: LOCK, RESTART）。
	// Event: トリガー識別子。
	common.SendPackByUUID(modules.Packet{Act: act, Event: trigger}, connUUID)

	//イベントリスナーの登録
	//AddEventOnce: デバイスからの応答を一度だけ処理するリスナーを登録。応答はトリガー識別子で紐づけられる。
	ok = common.AddEventOnce(func(p modules.Packet, _ *melody.Session) {
		/*
			レスポンス処理:
			失敗時 (p.Code != 0):
			ログに警告メッセージを記録。
			クライアントに 500 Internal Server Error を返す。
			成功時 (p.Code == 0):
			ログに成功情報を記録。
			クライアントに 200 OK を返す。
		*/
		if p.Code != 0 {
			common.Warn(ctx, `CALL_DEVICE`, `fail`, p.Msg, map[string]any{
				`act`: act,
			})
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		} else {
			common.Info(ctx, `CALL_DEVICE`, `success`, ``, map[string]any{
				`act`: act,
			})
			ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
		}
	}, connUUID, trigger, 5*time.Second)

	//タイムアウト処理
	//イベントリスナーが登録されなかった場合（クライアントがオフラインと推定）:
	// デバイスが応答できないため、「成功」と見なして 200 OK を返す。
	// ログに情報メッセージを記録。
	if !ok {
		//This means the client is offline.
		//So we take this as a success.
		common.Info(ctx, `CALL_DEVICE`, `success`, ``, map[string]any{
			`act`: act,
		})
		ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
	}

	/*
		全体の処理フロー
		アクションとデバイスの検証:
		クライアントが指定したアクションとターゲットデバイスの有効性を確認。
		アクションの送信:
		デバイスに対してアクションリクエストを送信。
		レスポンスの処理:
		デバイスからの応答を受信して処理。
		応答がない場合（タイムアウト）、成功と見なして処理を終了。

		このコードの特徴
		柔軟なアクション管理:
		許可されたアクションをリスト化して簡単に管理可能。
		非同期応答処理:
		イベント駆動設計により、デバイスの応答を効率的に処理。
		タイムアウト対応:
		デバイスがオフラインの場合も適切に処理。
		セキュアな設計:
		デバイスやアクションの検証が組み込まれており、不正なリクエストを防止。
	*/
}

/*
説明: データをセッションごとに一意な「Secret」を使用してシンプルなXOR暗号化を行います。
機能:
セッションの Secret を使用して、データをXOR方式で暗号化または復号化します。
*/
func SimpleEncrypt(data []byte, session *melody.Session) []byte {
	temp, ok := session.Get(`Secret`)
	if !ok {
		return nil
	}
	secret := temp.([]byte)
	return utils.XOR(data, secret)
}

func SimpleDecrypt(data []byte, session *melody.Session) []byte {
	temp, ok := session.Get(`Secret`)
	if !ok {
		return nil
	}
	secret := temp.([]byte)
	return utils.XOR(data, secret)
}

/*
説明: WebSocket接続のヘルスチェックを行います。
機能:
一定期間応答がないWebSocketセッションをクローズします。
各セッションに対して PING パケットを送信し、応答がないセッションをクローズします。
最後のメッセージが受信されてから300秒以上経過したセッションも終了させます。
*/
func WSHealthCheck(container *melody.Melody, sender Sender) {
	const MaxIdleSeconds = 300
	ping := func(uuid string, s *melody.Session) {
		if !sender(modules.Packet{Act: `PING`}, s) {
			s.Close()
		}
	}
	// 定期的にpingを実行し、疎通を確認する
	for now := range time.NewTicker(60 * time.Second).C {
		timestamp := now.Unix()
		// stores sessions to be disconnected
		queue := make([]*melody.Session, 0)

		//すべてのセッションに実行
		container.IterSessions(func(uuid string, s *melody.Session) bool {
			// pingの実行
			go ping(uuid, s)

			// 最後のパケットを確認
			val, ok := s.Get(`LastPack`)
			// 存在しない場合、削除キューに追加
			if !ok {
				queue = append(queue, s)
				return true
			}
			//パケットが不正の場合、削除キューに追加
			lastPack, ok := val.(int64)
			if !ok {
				queue = append(queue, s)
				return true
			}
			// 最後のパケットの時間から300秒以上経過している場合もタイムアウトとする
			if timestamp-lastPack > MaxIdleSeconds {
				queue = append(queue, s)
			}
			return true
		})

		// セッションを閉じる
		for i := 0; i < len(queue); i++ {
			queue[i].Close()
		}
	}
}
