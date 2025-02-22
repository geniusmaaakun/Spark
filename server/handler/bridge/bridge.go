package bridge

import (
	"Spark/modules"
	"Spark/utils"
	"Spark/utils/cmap"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

/*
クライアントとブラウザ間でバイナリデータを転送するための「Bridge（橋渡し）」を実装したものです。
Bridgeは、2つの異なるコンテキスト（クライアントとブラウザ）の間でバイナリデータを中継し、セッション管理やデータの読み書き、タイムアウト処理などを行います。Ginフレームワークを利用して、HTTPリクエストとレスポンスを扱っています。

クライアントとブラウザの間でバイナリデータを効率的に転送するための仕組みを提供しています。Bridgeを使用して、クライアント（Src）とブラウザ（Dst）間でデータを中継し、通信が終了したらリソースを解放します。また、タイムアウト処理やガベージコレクションにより、メモリリークを防ぎます。

*/

// Bridge is a utility that handles the binary flow from the client
// to the browser or flow from the browser to the client.

/*
creation: このBridgeが作成されたタイムスタンプ（UNIX時間）。
using: 現在このBridgeが使用中かどうかを示すフラグ。
uuid: ブリッジを一意に識別するためのUUID。
lock: スレッドセーフに処理を行うためのミューテックスロック。
Dst: データの送信先となるコンテキスト（通常はブラウザ）。
Src: データの送信元となるコンテキスト（通常はクライアント）。
ext: 拡張情報（任意のデータ型を保持できるフィールド）。
OnPull: ブリッジの「Pull」（データを受信する側）操作時に呼ばれるコールバック関数。
OnPush: ブリッジの「Push」（データを送信する側）操作時に呼ばれるコールバック関数。
OnFinish: ブリッジの処理が終了したときに呼ばれるコールバック関数。
*/
type Bridge struct {
	creation int64
	using    bool
	uuid     string
	lock     *sync.Mutex
	Dst      *gin.Context
	Src      *gin.Context
	ext      any
	OnPull   func(bridge *Bridge)
	OnPush   func(bridge *Bridge)
	OnFinish func(bridge *Bridge)
}

// すべてのBridgeインスタンスをUUIDで管理するスレッドセーフなマップ。このマップにはアクティブなBridgeインスタンスが格納され、セッション管理を行います。
var bridges = cmap.New[*Bridge]()

// このinit関数は、15秒ごとに定期的にbridgesの内容を確認し、60秒以上使用されていないブリッジを削除するガベージコレクション的な役割を果たします。古いブリッジを削除してメモリを解放します。
func init() {
	go func() {
		for now := range time.NewTicker(15 * time.Second).C {
			var queue []string
			timestamp := now.Unix()
			// 要素に対して使用しているかを確認
			bridges.IterCb(func(k string, b *Bridge) bool {
				// 使用の確認
				if timestamp-b.creation > 60 && !b.using {
					b.lock.Lock()
					if b.Src != nil && b.Src.Request.Body != nil {
						b.Src.Request.Body.Close()
					}
					b.Src = nil
					b.Dst = nil
					b.lock.Unlock()
					b = nil
					// 削除キューに追加
					queue = append(queue, b.uuid)
				}
				return true
			})
			bridges.Remove(queue...)
		}
	}()
}

// **CheckBridge**は、リクエストで提供されたブリッジID（form.Bridge）を元に、対応するBridgeインスタンスを取得します。もしブリッジが見つからない場合は、400 Bad Requestエラーを返します。
func CheckBridge(ctx *gin.Context) *Bridge {
	var form struct {
		Bridge string `json:"bridge" yaml:"bridge" form:"bridge" binding:"required"`
	}
	//リクエストからbridgeフィールドを抽出してform構造体にマッピングします。
	//マッピングに失敗した場合（リクエストにbridgeフィールドが欠けている、または形式が無効な場合）はエラーを返します。
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return nil
	}
	// インスタンスを取得
	b, ok := bridges.Get(form.Bridge)
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_BRIDGE_ID}`})
		return nil
	}
	return b
}

/*
BridgePushは、クライアントからブラウザへのデータの送信操作を処理します。
**CheckBridge**を使って、リクエストが有効なブリッジに関連しているか確認します。
もしブリッジがすでに使用中であれば、エラーレスポンスを返して処理を終了します。
ブリッジが使用可能であれば、OnPushコールバックを呼び出してからデータの転送を開始します。
*/
func BridgePush(ctx *gin.Context) {
	//クライアントのリクエストに基づき、有効なブリッジIDを検証します。
	bridge := CheckBridge(ctx)
	if bridge == nil {
		return
	}
	bridge.lock.Lock()
	//使用中のブリッジのチェック:
	//bridge.usingがtrue、またはbridge.Srcとbridge.Dstの両方がすでに設定されている場合、そのブリッジは使用中とみなされます。
	if bridge.using || (bridge.Src != nil && bridge.Dst != nil) {
		bridge.lock.Unlock()
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: 1, Msg: `${i18n|COMMON.BRIDGE_IN_USE}`})
		return
	}
	//使用可能な場合、リクエスト（ctx）をbridge.Srcに設定し、bridge.usingをtrueに変更。
	bridge.Src = ctx
	bridge.using = true
	bridge.lock.Unlock()
	//ブリッジにOnPushコールバック関数が設定されている場合、それを実行。
	//このコールバックは、ブリッジがプッシュ（データ送信）操作を開始したときのカスタム処理を定義できます。
	if bridge.OnPush != nil {
		bridge.OnPush(bridge)
	}
	//送信先の確認:
	//bridge.DstとそのWriterが設定されている場合、データの転送を開始します。
	if bridge.Dst != nil && bridge.Dst.Writer != nil {
		//SrcConnとDstConnの取得:
		// クライアント（Src）と宛先（Dst）のネットワーク接続を取得。
		// 両方が有効である場合にのみ処理を続行。
		// Get net.Conn to set deadline manually.
		SrcConn, SrcOK := bridge.Src.Request.Context().Value(`Conn`).(net.Conn)
		DstConn, DstOK := bridge.Dst.Request.Context().Value(`Conn`).(net.Conn)
		if SrcOK && DstOK {
			for {
				eof := false
				buf := make([]byte, 2<<14)
				//クライアントからの読み込み（5秒）と宛先への書き込み（10秒）のタイムアウトを設定。
				SrcConn.SetReadDeadline(utils.Now.Add(5 * time.Second))
				//クライアントから32KBのデータを読み込み（Body.Read）、宛先に書き込む（Writer.Write）。
				n, err := bridge.Src.Request.Body.Read(buf)
				if n == 0 {
					break
				}
				//エラーが発生、またはEOF（データ終了）に到達した場合、ループを終了。
				if err != nil {
					eof = err == io.EOF
					if !eof {
						break
					}
				}
				DstConn.SetWriteDeadline(utils.Now.Add(10 * time.Second))
				_, err = bridge.Dst.Writer.Write(buf[:n])
				if eof || err != nil {
					break
				}
			}
		}

		//接続の終了

		//転送が終了した後、SrcConnとDstConnのタイムアウト設定をリセット
		SrcConn.SetReadDeadline(time.Time{})
		DstConn.SetWriteDeadline(time.Time{})

		//クライアントにHTTPステータス200 OKを送信。
		bridge.Src.Status(http.StatusOK)

		//ブリッジの終了処理が必要な場合はOnFinishコールバックを実行。
		if bridge.OnFinish != nil {
			bridge.OnFinish(bridge)
		}

		//RemoveBridgeを呼び出してブリッジを解放。
		RemoveBridge(bridge.uuid)
		bridge = nil
	}
}

/*
BridgePullは、ブラウザからクライアントへのデータの受信操作を処理します。
BridgePushと同様に、ブリッジの状態を確認しながらデータ転送を開始します。
*/
func BridgePull(ctx *gin.Context) {
	//クライアントのリクエストに基づいて、有効なブリッジIDを確認します。
	bridge := CheckBridge(ctx)
	if bridge == nil {
		return
	}
	bridge.lock.Lock()
	if bridge.using || (bridge.Src != nil && bridge.Dst != nil) {
		bridge.lock.Unlock()
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: 1, Msg: `${i18n|COMMON.BRIDGE_IN_USE}`})
		return
	}
	bridge.Dst = ctx
	bridge.using = true
	bridge.lock.Unlock()

	//ブリッジにOnPullコールバック関数が設定されている場合、それを実行します。
	//このコールバックは、データの受信操作が開始されたときに実行するカスタム処理を定義できます。
	if bridge.OnPull != nil {
		bridge.OnPull(bridge)
	}

	//クライアント（Src）が設定されており、そのリクエストボディ（Body）が存在する場合にのみ転送を開始します。
	if bridge.Src != nil && bridge.Src.Request.Body != nil {
		// Get net.Conn to set deadline manually.
		SrcConn, SrcOK := bridge.Src.Request.Context().Value(`Conn`).(net.Conn)
		DstConn, DstOK := bridge.Dst.Request.Context().Value(`Conn`).(net.Conn)
		if SrcOK && DstOK {
			for {
				eof := false
				buf := make([]byte, 2<<14)
				SrcConn.SetReadDeadline(utils.Now.Add(5 * time.Second))
				n, err := bridge.Src.Request.Body.Read(buf)
				if n == 0 {
					break
				}
				if err != nil {
					eof = err == io.EOF
					if !eof {
						break
					}
				}
				DstConn.SetWriteDeadline(utils.Now.Add(10 * time.Second))
				_, err = bridge.Dst.Writer.Write(buf[:n])
				if eof || err != nil {
					break
				}
			}
		}

		//
		SrcConn.SetReadDeadline(time.Time{})
		DstConn.SetWriteDeadline(time.Time{})
		bridge.Src.Status(http.StatusOK)
		if bridge.OnFinish != nil {
			bridge.OnFinish(bridge)
		}
		RemoveBridge(bridge.uuid)
		bridge = nil
	}
}

/*
AddBridge: 新しいブリッジを作成し、UUIDで識別してbridgesマップに保存します。
AddBridgeWithSrc / AddBridgeWithDst: SrcまたはDstを初期化してからブリッジを追加する関数です。
*/
func AddBridge(ext any, uuid string) *Bridge {
	bridge := &Bridge{
		creation: utils.Unix,
		uuid:     uuid,
		using:    false,
		lock:     &sync.Mutex{},
		ext:      ext,
	}
	bridges.Set(uuid, bridge)
	return bridge
}

func AddBridgeWithSrc(ext any, uuid string, Src *gin.Context) *Bridge {
	bridge := &Bridge{
		creation: utils.Unix,
		uuid:     uuid,
		using:    false,
		lock:     &sync.Mutex{},
		ext:      ext,
		Src:      Src,
	}
	bridges.Set(uuid, bridge)
	return bridge
}

func AddBridgeWithDst(ext any, uuid string, Dst *gin.Context) *Bridge {
	bridge := &Bridge{
		creation: utils.Unix,
		uuid:     uuid,
		using:    false,
		lock:     &sync.Mutex{},
		ext:      ext,
		Dst:      Dst,
	}
	bridges.Set(uuid, bridge)
	return bridge
}

/*
**RemoveBridge**は、UUIDで指定されたブリッジを削除し、リソースを解放します。送信元と送信先のリクエストボディも閉じて、メモリを解放します。
 */
func RemoveBridge(uuid string) {
	b, ok := bridges.Get(uuid)
	if !ok {
		return
	}
	bridges.Remove(uuid)
	if b.Src != nil && b.Src.Request.Body != nil {
		b.Src.Request.Body.Close()
	}
	b.Src = nil
	b.Dst = nil
	b = nil
}
