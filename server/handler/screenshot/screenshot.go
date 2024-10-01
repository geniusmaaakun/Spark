package screenshot

import (
	"Spark/modules"
	"Spark/server/common"
	"Spark/server/handler/bridge"
	"Spark/server/handler/utility"
	"Spark/utils"
	"Spark/utils/melody"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

/*
リモートクライアントからスクリーンショットを取得するためのAPIを実装しています。
クライアントにスクリーンショットのリクエストを送信し、取得した画像をブラウザに返します。また、リクエストに対する応答が5秒以内に得られなかった場合には、タイムアウトエラーを返します。

処理の概要
リクエストの検証: utility.CheckFormを使って、リクエストの内容を確認し、リモートクライアントの情報を取得します。
スクリーンショット要求: SendPackByUUIDを使って、リモートクライアントにスクリーンショットのリクエストを送信します。
データ受信処理: bridgeを使ってスクリーンショットデータを受信し、image/pngとしてクライアントに送信します。
エラーハンドリング: クライアントからの応答がない場合、タイムアウトエラーを返し、エラーメッセージを記録します。
このコードは、リモートクライアントからスクリーンショットを取得し、その画像をブラウザに表示する機能を提供しています。
*/

/*
関数の流れと役割
utility.CheckForm(ctx, nil)

リクエストの検証を行い、ターゲットのリモートデバイス（target）を取得します。失敗した場合は処理を中止します。
bridgeID と trigger の生成

それぞれリクエストごとにユニークなIDを生成します。
bridgeID はデータ転送用のブリッジを識別するIDで、trigger はイベントのトリガー用IDです。
SendPackByUUID

リモートクライアントにスクリーンショットのリクエスト（SCREENSHOT）を送信します。このリクエストにはbridgeIDも含まれます。
AddEvent

リモートクライアントからの応答を待ちます。応答が成功か失敗かに応じて処理が分かれます。
失敗時には、エラーメッセージを返し、500 Internal Server Error をクライアントに送信します。また、エラーログを記録します。
ブリッジ（データ転送用）の作成

bridge.AddBridgeWithDstを使ってブリッジを作成し、データの受信を開始します。
OnPush: リモートデバイスからスクリーンショットのデータが送信された際に呼び出されます。ヘッダーにContent-Type: image/pngを設定します。
OnFinish: データの送信が完了した際に呼び出され、成功ログを記録します。
タイムアウト処理

5秒以内にスクリーンショットが送信されなかった場合、504 Gateway Timeout を返し、エラーログを記録します。
*/
// GetScreenshot will call client to screenshot.
func GetScreenshot(ctx *gin.Context) {
	target, ok := utility.CheckForm(ctx, nil)
	if !ok {
		return
	}
	bridgeID := utils.GetStrUUID()
	trigger := utils.GetStrUUID()
	wait := make(chan bool)
	called := false
	common.SendPackByUUID(modules.Packet{Act: `SCREENSHOT`, Data: gin.H{`bridge`: bridgeID}, Event: trigger}, target)
	common.AddEvent(func(p modules.Packet, _ *melody.Session) {
		called = true
		bridge.RemoveBridge(bridgeID)
		common.RemoveEvent(trigger)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		common.Warn(ctx, `SCREENSHOT`, `fail`, p.Msg, nil)
		wait <- false
	}, target, trigger)
	instance := bridge.AddBridgeWithDst(nil, bridgeID, ctx)
	instance.OnPush = func(bridge *bridge.Bridge) {
		called = true
		common.RemoveEvent(trigger)
		ctx.Header(`Content-Type`, `image/png`)
	}
	instance.OnFinish = func(bridge *bridge.Bridge) {
		if called {
			common.Info(ctx, `SCREENSHOT`, `success`, ``, nil)
		}
		wait <- false
	}
	select {
	case <-wait:
	case <-time.After(5 * time.Second):
		if !called {
			bridge.RemoveBridge(bridgeID)
			common.RemoveEvent(trigger)
			ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
			common.Warn(ctx, `SCREENSHOT`, `fail`, `timeout`, nil)
		} else {
			<-wait
		}
	}
	close(wait)
}
