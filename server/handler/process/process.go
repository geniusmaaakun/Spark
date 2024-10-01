package process

import (
	"Spark/modules"
	"Spark/server/common"
	"Spark/server/handler/utility"
	"Spark/utils"
	"Spark/utils/melody"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

/*
リモートクライアントのプロセス管理を行うためのAPIを提供しています。
リモートデバイス上で実行中のプロセスをリスト表示したり、特定のプロセスを終了させる機能を実装しています。以下に各関数の詳細な解説を行います。


ListDeviceProcesses 関数はリモートデバイス上のプロセス一覧を取得し、クライアントに返します。
KillDeviceProcess 関数は指定されたプロセスIDを元にリモートデバイス上のプロセスを終了させます。
両関数とも、リクエストごとにユニークなイベントIDを生成し、リモートデバイスにリクエストを送信します。リモートデバイスからの応答を待つために、5秒間のタイムアウトが設定されています。
*/

// ListDeviceProcesses will list processes on remote client
/*
この関数は、リモートデバイス上で実行されているプロセスのリストを取得します。

処理内容:
CheckForm 関数を使ってリクエストの検証を行います（connUUIDを取得します）。検証に失敗した場合は処理を中止します。
trigger: リクエストごとにユニークなイベントIDを生成します。
SendPackByUUID: リモートデバイスに対してプロセスリストを取得するリクエスト（PROCESSES_LIST）を送信します。
AddEventOnce: リモートデバイスからの応答を待ちます。
リモートデバイスが成功の応答を返した場合、HTTP 200 OK と共にプロセスリストを返します。
エラーが発生した場合、HTTP 500 Internal Server Error を返します。
タイムアウト（5秒以内に応答がない場合）時には、504 Gateway Timeout を返します。

*/
func ListDeviceProcesses(ctx *gin.Context) {
	connUUID, ok := utility.CheckForm(ctx, nil)
	if !ok {
		return
	}
	trigger := utils.GetStrUUID()
	common.SendPackByUUID(modules.Packet{Act: `PROCESSES_LIST`, Event: trigger}, connUUID)
	ok = common.AddEventOnce(func(p modules.Packet, _ *melody.Session) {
		if p.Code != 0 {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		} else {
			ctx.JSON(http.StatusOK, modules.Packet{Code: 0, Data: p.Data})
		}
	}, connUUID, trigger, 5*time.Second)
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
	}
}

// KillDeviceProcess will try to get send a packet to
// client and let it kill the process specified.
/*
役割:
この関数は、リモートデバイス上で特定のプロセス（pidで指定されたプロセス）を終了させます。

処理内容:
CheckForm 関数でリクエストの検証を行い、プロセスID（pid）を取得します。
trigger: リクエストごとにユニークなイベントIDを生成します。
SendPackByUUID: リモートデバイスに対してプロセス終了リクエスト（PROCESS_KILL）を送信します。
AddEventOnce: リモートデバイスからの応答を待ちます。
リモートデバイスがプロセス終了に成功した場合、HTTP 200 OK と共に成功を通知します。
エラーが発生した場合、HTTP 500 Internal Server Error を返し、エラーメッセージを記録します。
タイムアウト（5秒以内に応答がない場合）時には、504 Gateway Timeout を返し、タイムアウトを警告します。
*/
func KillDeviceProcess(ctx *gin.Context) {
	var form struct {
		Pid int32 `json:"pid" yaml:"pid" form:"pid" binding:"required"`
	}
	target, ok := utility.CheckForm(ctx, &form)
	if !ok {
		return
	}
	trigger := utils.GetStrUUID()
	common.SendPackByUUID(modules.Packet{Act: `PROCESS_KILL`, Data: gin.H{`pid`: form.Pid}, Event: trigger}, target)
	ok = common.AddEventOnce(func(p modules.Packet, _ *melody.Session) {
		if p.Code != 0 {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
			common.Warn(ctx, `PROCESS_KILL`, `fail`, p.Msg, map[string]any{
				`pid`: form.Pid,
			})
		} else {
			ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
			common.Info(ctx, `PROCESS_KILL`, `success`, ``, map[string]any{
				`pid`: form.Pid,
			})
		}
	}, target, trigger, 5*time.Second)
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
		common.Warn(ctx, `PROCESS_KILL`, `fail`, `timeout`, map[string]any{
			`pid`: form.Pid,
		})
	}
}
