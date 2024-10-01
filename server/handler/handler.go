package handler

import (
	"Spark/server/handler/bridge"
	"Spark/server/handler/desktop"
	"Spark/server/handler/file"
	"Spark/server/handler/generate"
	"Spark/server/handler/process"
	"Spark/server/handler/screenshot"
	"Spark/server/handler/terminal"
	"Spark/server/handler/utility"

	"github.com/gin-gonic/gin"
)

/*
Webアプリケーション内で複数のリモート操作を行うためのAPIエンドポイントを設定します。主にリモートデバイスとやり取りし、ファイル管理、プロセス管理、スクリーンショット取得、ターミナル接続、デスクトップ接続などをサポートしています。
*/

var AuthHandler gin.HandlerFunc

// InitRouter will initialize http and websocket routers.
func InitRouter(ctx *gin.RouterGroup) {
	/*
		/bridge/push と /bridge/pull: WebSocketを使用したブリッジング機能。クライアントからのデータの送信・受信を処理します（bridge パッケージ）。
		/client/update: クライアントのバージョンチェックと更新を行います（utility.CheckUpdate 関数）。
	*/
	ctx.Any(`/bridge/push`, bridge.BridgePush)
	ctx.Any(`/bridge/pull`, bridge.BridgePull)
	ctx.Any(`/client/update`, utility.CheckUpdate) // Client, for update.

	/*
		グループ化された認証が必要なルート:
		スクリーンショット取得:
		POST /device/screenshot/get: リモートデバイスのスクリーンショットを取得します。
		プロセス管理:
		POST /device/process/list: リモートデバイス上のプロセス一覧を取得します。
		POST /device/process/kill: リモートデバイス上のプロセスを終了します。
		ファイル操作:
		POST /device/file/remove: リモートデバイスからファイルを削除します。
		POST /device/file/upload: リモートデバイスにファイルをアップロードします。
		POST /device/file/list: リモートデバイスのファイル一覧を取得します。
		POST /device/file/text: リモートデバイスのテキストファイルを取得します。
		POST /device/file/get: リモートデバイスからファイルをダウンロードします。
		コマンド実行:
		POST /device/exec: リモートデバイス上でコマンドを実行します。
		デバイス管理:
		POST /device/list: 接続されているデバイスの一覧を取得します。
		POST /device/:act: デバイスの特定のアクション（例: ロック、ログオフ、再起動、シャットダウンなど）を実行します。
		クライアント生成:
		POST /client/check: クライアントのチェックを行います（generate.CheckClient 関数）。
		POST /client/generate: クライアントの生成を行います（generate.GenerateClient 関数）。
		ターミナル・デスクトップ接続:
		Any /device/terminal: WebSocketを使用してターミナルセッションを初期化します。
		Any /device/desktop: WebSocketを使用してデスクトップセッションを初期化します。
	*/
	group := ctx.Group(`/`, AuthHandler)
	{
		group.POST(`/device/screenshot/get`, screenshot.GetScreenshot)
		group.POST(`/device/process/list`, process.ListDeviceProcesses)
		group.POST(`/device/process/kill`, process.KillDeviceProcess)
		group.POST(`/device/file/remove`, file.RemoveDeviceFiles)
		group.POST(`/device/file/upload`, file.UploadToDevice)
		group.POST(`/device/file/list`, file.ListDeviceFiles)
		group.POST(`/device/file/text`, file.GetDeviceTextFile)
		group.POST(`/device/file/get`, file.GetDeviceFiles)
		group.POST(`/device/exec`, utility.ExecDeviceCmd)
		group.POST(`/device/list`, utility.GetDevices)
		group.POST(`/device/:act`, utility.CallDevice)
		group.POST(`/client/check`, generate.CheckClient)
		group.POST(`/client/generate`, generate.GenerateClient)
		group.Any(`/device/terminal`, terminal.InitTerminal)
		group.Any(`/device/desktop`, desktop.InitDesktop)
	}
}
