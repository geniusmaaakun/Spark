package main

import (
	"Spark/modules"
	"Spark/server/auth"
	"Spark/server/common"
	"Spark/server/config"
	"Spark/server/handler"
	"Spark/server/handler/desktop"
	"Spark/server/handler/terminal"
	"Spark/server/handler/utility"
	"Spark/utils/cmap"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/rakyll/statik/fs"

	_ "Spark/server/embed/web"
	"Spark/utils"
	"Spark/utils/melody"
	"net/http"

	"github.com/gin-gonic/gin"
)

/*
Goの Ginフレームワーク を使用して構築された Webサーバー です。
このサーバーは、クライアントとの WebSocket 通信を管理し、リモートデバイスからのメッセージやコマンドを処理するためのAPIエンドポイントを提供します。
また、ユーザー認証やセキュリティ対策、ファイルのキャッシュ管理も行っています。


このコードは、クライアントとの リアルタイム通信 を確立し、メッセージやファイルの転送、デバイス管理などを行うサーバーの構築方法を示しています。主な機能は以下のとおりです：

WebSocket接続のハンドリング: クライアントとのリアルタイム通信。
ファイルキャッシュの管理: 静的ファイルを効率的に配信するためのキャッシュ制御。
認証の管理: 認証が必要なAPIエンドポイントへのアクセス制御。
デバイス管理: リモートデバイスからのコマンドの受信・処理。



リアルタイム通信をサポートし、リモートデバイス管理に必要な各種機能を提供します。WebSocketを使用してクライアントからのバイナリデータやコマンドを処理し、認証やキャッシュ管理、デバイスのPing応答の監視なども行います。
*/

/*
の blocked マップは、キーとして文字列（通常はIPアドレス）、値として int64 型のデータを保持するデータ構造です。

具体的な用途
この blocked 変数は、IPアドレスなどの特定のキーに対して、そのアドレスが 一時的にブロックされているかどうか を管理するために使用されます。ここでの int64 は、そのアドレスがブロックされている期間の終了時刻を示しており、ブロックが解除されるまでの残り時間を管理します。

使用例
この blocked マップを使って、リクエストを送信したクライアントのIPアドレスが過剰なリクエストを送信していないかを確認し、必要に応じて一定時間ブロックします。ブロックされたクライアントのIPアドレスとそのブロックが解除される時刻（int64 型のUNIXタイムスタンプ）を保存します。

例えば：
あるクライアントが多くの失敗した認証試行を行うと、そのクライアントのIPアドレスが blocked に追加され、一定期間そのクライアントからのリクエストがブロックされます。
blocked に保存されている値を定期的にチェックし、ブロック解除のタイミングが来たらそのエントリを削除します。
*/

// IP アドレスを保持する。認証に失敗したら追加する
var blocked = cmap.New[int64]()

// ?
var lastRequest = time.Now().Unix()

/*
説明:
サーバーのエントリーポイントです。以下の手順でサーバーをセットアップしています。
静的リソースの読み込み (webFS): サーバーが提供するWebコンテンツ（HTML/CSS/JSファイルなど）を読み込みます。
ルーティングの初期化 (handler.InitRouter): /api パスの下にあるAPIエンドポイントを初期化し、クライアントとのWebSocket接続のための /ws エンドポイントも設定します。
WebSocketのハンドリング (wsOnConnect, wsOnMessage, wsOnMessageBinary, wsOnDisconnect): WebSocket接続のイベントを処理します。
HTTPサーバーの起動 (srv.ListenAndServe): 指定されたポートでHTTPサーバーを起動します。
シグナル処理: SIGINTやSIGTERMシグナルをキャッチし、サーバーを安全にシャットダウンします。
*/
func main() {
	webFS, err := fs.NewWithNamespace(`web`)
	if err != nil {
		common.Fatal(nil, `LOAD_STATIC_RES`, `fail`, err.Error(), nil)
		return
	}
	gin.SetMode(gin.ReleaseMode)
	app := gin.New()
	app.Use(gin.Recovery())
	{
		handler.AuthHandler = checkAuth()
		handler.InitRouter(app.Group(`/api`))
		app.Any(`/ws`, wsHandshake)
		app.NoRoute(handler.AuthHandler, func(ctx *gin.Context) {
			if !serveGzip(ctx, webFS) && !checkCache(ctx, webFS) {
				http.FileServer(webFS).ServeHTTP(ctx.Writer, ctx.Request)
			}
		})
	}

	common.Melody.Config.MaxMessageSize = common.MaxMessageSize
	common.Melody.HandleConnect(wsOnConnect)
	common.Melody.HandleMessage(wsOnMessage)
	common.Melody.HandleMessageBinary(wsOnMessageBinary)
	common.Melody.HandleDisconnect(wsOnDisconnect)
	go wsHealthCheck(common.Melody)

	srv := &http.Server{
		Addr:    config.Config.Listen,
		Handler: app,
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			ctx = context.WithValue(ctx, `Conn`, c)
			ctx = context.WithValue(ctx, `ClientIP`, common.GetAddrIP(c.RemoteAddr()))
			return ctx
		},
	}
	{
		go func() {
			err = srv.ListenAndServe()
		}()
		if err != nil {
			common.Fatal(nil, `SERVICE_INIT`, `fail`, err.Error(), nil)
		} else {
			common.Info(nil, `SERVICE_INIT`, ``, ``, map[string]any{
				`listen`: config.Config.Listen,
			})
		}
	}
	quit := make(chan os.Signal, 3)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	common.Warn(nil, `SERVICE_EXITING`, ``, ``, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		common.Warn(nil, `SERVICE_EXIT`, `error`, err.Error(), nil)
	}
	<-ctx.Done()
	common.Warn(nil, `SERVICE_EXIT`, `success`, ``, nil)
	common.CloseLog()
}

/*
説明: WebSocket接続のハンドシェイクを処理します。認証情報（UUIDとKey）をチェックし、クライアントからのWebSocket接続を初期化します。
クライアントがWebSocketではなく通常のHTTPリクエストを使用した場合は、そのリクエストに対して応答します（例: 大きすぎるメッセージの場合）。
*/
func wsHandshake(ctx *gin.Context) {
	if !ctx.IsWebsocket() {
		// When message is too large to transport via websocket,
		// client will try to send these data via http.
		const MaxBodySize = 2 << 18 // 524288 512KB
		if ctx.Request.ContentLength > MaxBodySize {
			ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, modules.Packet{Code: 1})
			return
		}
		body, err := ctx.GetRawData()
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: 1})
			return
		}
		session := common.CheckClientReq(ctx)
		if session == nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, modules.Packet{Code: 1})
			return
		}
		wsOnMessageBinary(session, body)
		ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
		return
	}

	clientUUID, _ := hex.DecodeString(ctx.GetHeader(`UUID`))
	clientKey, _ := hex.DecodeString(ctx.GetHeader(`Key`))
	if len(clientUUID) != 16 || len(clientKey) != 32 {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	decrypted, err := common.DecAES(clientKey, config.Config.SaltBytes)
	if err != nil || !bytes.Equal(decrypted, clientUUID) {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	secret := append(utils.GetUUID(), utils.GetUUID()...)
	ctx.Writer.Header().Add(`Secret`, hex.EncodeToString(secret))
	err = common.Melody.HandleRequestWithKeys(ctx.Writer, ctx.Request, gin.H{
		`Secret`:   secret,
		`LastPack`: utils.Unix,
		`Address`:  common.GetRemoteAddr(ctx),
	})
	if err != nil {
		ctx.AbortWithStatus(http.StatusBadRequest)
		return
	}
}

/*
説明: クライアントがWebSocketに接続した際の処理を行います。デバイスにPingメッセージを送信します。
*/
func wsOnConnect(session *melody.Session) {
	pingDevice(session)
}

/*
説明: テキストメッセージを受信したときの処理を行います。ここでは特定の処理は行わず、クライアントを切断します。
*/
func wsOnMessage(session *melody.Session, _ []byte) {
	session.Close()
}

/*
説明: バイナリデータのメッセージを受信したときの処理を行います。
データの解析: 受信したバイナリデータを解析し、特定のサービス（例: 20 や 21）に基づいて適切なイベントを呼び出します。
パケットの復号と解析: AES暗号化されたデータを復号し、パケットが有効かどうかを確認します。
*/
func wsOnMessageBinary(session *melody.Session, data []byte) {
	var pack modules.Packet

	dataLen := len(data)
	if dataLen > 24 {
		if service, op, isBinary := utils.CheckBinaryPack(data); isBinary {
			switch service {
			case 20:
				switch op {
				case 00, 01, 02, 03:
					event := hex.EncodeToString(data[6:22])
					copy(data[6:], data[22:])
					common.CallEvent(modules.Packet{
						Act:   `RAW_DATA_ARRIVE`,
						Event: event,
						Data: gin.H{
							`data`: utils.GetSlicePrefix(&data, dataLen-16),
						},
					}, session)
				}
			case 21:
				switch op {
				case 00, 01:
					event := hex.EncodeToString(data[6:22])
					copy(data[6:], data[22:])
					common.CallEvent(modules.Packet{
						Act:   `RAW_DATA_ARRIVE`,
						Event: event,
						Data: gin.H{
							`data`: utils.GetSlicePrefix(&data, dataLen-16),
						},
					}, session)
				}
			}
			return
		}
	}

	data, ok := common.Decrypt(data, session)
	if !(ok && utils.JSON.Unmarshal(data, &pack) == nil) {
		common.SendPack(modules.Packet{Code: -1}, session)
		session.CloseWithMsg(melody.FormatCloseMessage(1000, `invalid request`))
		return
	}
	if pack.Act == `DEVICE_UP` || pack.Act == `DEVICE_UPDATE` {
		session.Set(`LastPack`, utils.Unix)
		utility.OnDevicePack(data, session)
		return
	}
	if !common.Devices.Has(session.UUID) {
		session.CloseWithMsg(melody.FormatCloseMessage(1001, `invalid device id`))
		return
	}
	common.CallEvent(pack, session)
	session.Set(`LastPack`, utils.Unix)
}

/*
説明: クライアントがWebSocketから切断された際の処理を行います。デバイス情報を削除し、ターミナルやデスクトップセッションを閉じます。
*/
func wsOnDisconnect(session *melody.Session) {
	if device, ok := common.Devices.Get(session.UUID); ok {
		terminal.CloseSessionsByDevice(device.ID)
		desktop.CloseSessionsByDevice(device.ID)
		common.Info(nil, `CLIENT_OFFLINE`, ``, ``, map[string]any{
			`device`: map[string]any{
				`name`: device.Hostname,
				`ip`:   device.WAN,
			},
		})
	} else {
		common.Info(nil, `CLIENT_OFFLINE`, ``, ``, map[string]any{
			`device`: map[string]any{
				`ip`: common.GetAddrIP(session.GetWSConn().UnderlyingConn().RemoteAddr()),
			},
		})
	}
	common.Devices.Remove(session.UUID)
}

// 説明: 一定間隔でクライアントにPingメッセージを送信し、応答がないクライアントを切断します。
func wsHealthCheck(container *melody.Melody) {
	const MaxIdleSeconds = 150
	const MaxPingInterval = 60
	go func() {
		// Ping clients with a dynamic interval.
		// Interval will be greater than 3 seconds and less than MaxPingInterval.
		var tick int64 = 0
		var pingInterval int64 = 3
		for range time.NewTicker(3 * time.Second).C {
			tick += 3
			if tick >= utils.Unix-lastRequest {
				pingInterval = 3
			}
			if tick >= 3 && (tick >= pingInterval || tick >= MaxPingInterval) {
				pingInterval += 3
				if pingInterval > MaxPingInterval {
					pingInterval = MaxPingInterval
				}
				tick = 0
				container.IterSessions(func(uuid string, s *melody.Session) bool {
					go pingDevice(s)
					return true
				})
			}
		}
	}()
	for now := range time.NewTicker(60 * time.Second).C {
		timestamp := now.Unix()
		// Store sessions to be disconnected.
		queue := make([]*melody.Session, 0)
		container.IterSessions(func(uuid string, s *melody.Session) bool {
			val, ok := s.Get(`LastPack`)
			if !ok {
				queue = append(queue, s)
				return true
			}
			lastPack, ok := val.(int64)
			if !ok {
				queue = append(queue, s)
				return true
			}
			if timestamp-lastPack > MaxIdleSeconds {
				queue = append(queue, s)
			}
			return true
		})
		for i := 0; i < len(queue); i++ {
			queue[i].Close()
		}
	}
}

// 説明: 個別のデバイスにPingを送り、応答時間（レイテンシ）を計測します。
func pingDevice(s *melody.Session) {
	t := time.Now().UnixMilli()
	trigger := utils.GetStrUUID()
	common.SendPack(modules.Packet{Act: `PING`, Event: trigger}, s)
	common.AddEventOnce(func(packet modules.Packet, session *melody.Session) {
		device, ok := common.Devices.Get(s.UUID)
		if ok {
			device.Latency = uint(time.Now().UnixMilli()-t) / 2
		}
	}, s.UUID, trigger, 3*time.Second)
}

/*
説明: 認証を行うハンドラーファンクションを返します。
クッキー: Authorization クッキーをチェックし、既に認証済みか確認します。
Basic認証: 認証されていない場合、Basic認証を行い、成功したら Authorization クッキーをセットします。
ブロックリスト: 認証に失敗したクライアントを一時的にブロックします。
*/
func checkAuth() gin.HandlerFunc {
	// Token as key and update timestamp as value.
	// Stores authenticated tokens.
	tokens := cmap.New[int64]()
	go func() {
		for now := range time.NewTicker(60 * time.Second).C {
			var queue []string
			tokens.IterCb(func(key string, t int64) bool {
				if now.Unix()-t > 1800 {
					queue = append(queue, key)
				}
				return true
			})
			tokens.Remove(queue...)
			queue = nil

			blocked.IterCb(func(addr string, t int64) bool {
				if now.Unix() > t {
					queue = append(queue, addr)
				}
				return true
			})
			blocked.Remove(queue...)
		}
	}()

	if config.Config.Auth == nil || len(config.Config.Auth) == 0 {
		return func(ctx *gin.Context) {
			lastRequest = utils.Unix
			ctx.Next()
		}
	}

	auth := auth.BasicAuth(config.Config.Auth, ``)
	return func(ctx *gin.Context) {
		now := utils.Unix
		passed := false

		if token, err := ctx.Cookie(`Authorization`); err == nil {
			if tokens.Has(token) {
				lastRequest = now
				tokens.Set(token, now)
				passed = true
				return
			}
		}

		if !passed {
			addr := common.GetRealIP(ctx)
			if expire, ok := blocked.Get(addr); ok {
				if now < expire {
					ctx.AbortWithStatusJSON(http.StatusTooManyRequests, modules.Packet{Code: 1})
					return
				}
				blocked.Remove(addr)
			}

			auth(ctx)
			user := ctx.GetString(`user`)

			if ctx.IsAborted() {
				blocked.Set(addr, now+1)
				user = utils.If(len(user) == 0, `<EMPTY>`, user)
				common.Warn(ctx, `LOGIN_ATTEMPT`, `fail`, ``, map[string]any{
					`user`: user,
				})
				return
			}

			common.Warn(ctx, `LOGIN_ATTEMPT`, `success`, ``, map[string]any{
				`user`: user,
			})
			token := utils.GetStrUUID()
			tokens.Set(token, now)
			ctx.Header(`Set-Cookie`, fmt.Sprintf(`Authorization=%s; Path=/; HttpOnly`, token))
		}
		lastRequest = now
	}
}

// 説明: クライアントが gzip圧縮 に対応しているか確認し、対応していればgzip圧縮された静的ファイルを提供します。
func serveGzip(ctx *gin.Context, statikFS http.FileSystem) bool {
	headers := ctx.Request.Header
	filename := path.Clean(ctx.Request.RequestURI)
	if !strings.Contains(headers.Get(`Accept-Encoding`), `gzip`) {
		return false
	}
	if strings.Contains(headers.Get(`Connection`), `Upgrade`) {
		return false
	}
	if strings.Contains(headers.Get(`Accept`), `text/event-stream`) {
		return false
	}

	file, err := statikFS.Open(filename + `.gz`)
	if err != nil {
		return false
	}
	defer file.Close()

	file.Seek(0, io.SeekStart)
	conn, ok := ctx.Request.Context().Value(`Conn`).(net.Conn)
	if !ok {
		return false
	}

	etag := fmt.Sprintf(`"%x-%s"`, []byte(filename), config.COMMIT)
	if headers.Get(`If-None-Match`) == etag {
		ctx.Status(http.StatusNotModified)
		return true
	}
	ctx.Header(`Cache-Control`, `max-age=604800`)
	ctx.Header(`ETag`, etag)
	ctx.Header(`Expires`, utils.Now.Add(7*24*time.Hour).Format(`Mon, 02 Jan 2006 15:04:05 GMT`))

	ctx.Writer.Header().Del(`Content-Length`)
	ctx.Header(`Content-Encoding`, `gzip`)
	ctx.Header(`Vary`, `Accept-Encoding`)
	ctx.Status(http.StatusOK)

	for {
		eof := false
		buf := make([]byte, 2<<14)
		n, err := file.Read(buf)
		if n == 0 {
			break
		}
		if err != nil {
			eof = err == io.EOF
			if !eof {
				break
			}
		}
		conn.SetWriteDeadline(utils.Now.Add(10 * time.Second))
		_, err = ctx.Writer.Write(buf[:n])
		if eof || err != nil {
			break
		}
	}
	conn.SetWriteDeadline(time.Time{})
	ctx.Done()
	return true
}

/*
説明: キャッシュが有効かどうかを確認し、キャッシュが有効であれば304 Not Modifiedステータスを返します。
*/
func checkCache(ctx *gin.Context, _ http.FileSystem) bool {
	filename := path.Clean(ctx.Request.RequestURI)

	etag := fmt.Sprintf(`"%x-%s"`, []byte(filename), config.COMMIT)
	if ctx.Request.Header.Get(`If-None-Match`) == etag {
		ctx.Status(http.StatusNotModified)
		return true
	}
	ctx.Header(`ETag`, etag)
	ctx.Header(`Cache-Control`, `max-age=604800`)
	ctx.Header(`Expires`, utils.Now.Add(7*24*time.Hour).Format(`Mon, 02 Jan 2006 15:04:05 GMT`))
	return false
}
