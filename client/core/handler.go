package core

import (
	"Spark/client/common"
	"Spark/client/service/basic"
	"Spark/client/service/desktop"
	"Spark/client/service/file"
	"Spark/client/service/process"
	Screenshot "Spark/client/service/screenshot"
	"Spark/client/service/terminal"
	"Spark/modules"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"github.com/kataras/golog"
)

/*
クライアントサイドのWebSocket通信を通じて、さまざまなリモート操作を実行するためのハンドラ群を定義しています。クライアントはサーバーからのコマンドを受け取り、指定された操作（スクリーンショットの取得、プロセスの一覧表示、ファイル操作など）を実行します。

全体の流れ
WebSocketコネクションの確立: クライアントとサーバーの間でWebSocketを使って通信を行います。サーバーから送信されたデータ（Packet）を受け取り、適切なハンドラに渡します。
ハンドラ群: 各コマンドに対して対応する関数（ハンドラ）が定義されています。例えば、ping コマンドが送られてきた場合は ping 関数が呼ばれ、クライアントの状態をサーバーに報告します。
コールバック: ハンドラが実行された後、サーバーに成功または失敗のステータスを返します。

リモート管理ソフトウェアのクライアント側の実装であり、サーバーからの指示に従ってさまざまなシステム操作（電源管理、ファイル管理、ターミナル操作、プロセス管理など）を行うための処理を担当しています。
*/

var handlers = map[string]func(pack modules.Packet, wsConn *common.Conn){
	`PING`:             ping,
	`OFFLINE`:          offline,
	`LOCK`:             lock,
	`LOGOFF`:           logoff,
	`HIBERNATE`:        hibernate,
	`SUSPEND`:          suspend,
	`RESTART`:          restart,
	`SHUTDOWN`:         shutdown,
	`SCREENSHOT`:       screenshot,
	`TERMINAL_INIT`:    initTerminal,
	`TERMINAL_INPUT`:   inputTerminal,
	`TERMINAL_RESIZE`:  resizeTerminal,
	`TERMINAL_PING`:    pingTerminal,
	`TERMINAL_KILL`:    killTerminal,
	`FILES_LIST`:       listFiles,
	`FILES_FETCH`:      fetchFile,
	`FILES_REMOVE`:     removeFiles,
	`FILES_UPLOAD`:     uploadFiles,
	`FILE_UPLOAD_TEXT`: uploadTextFile,
	`PROCESSES_LIST`:   listProcesses,
	`PROCESS_KILL`:     killProcess,
	`DESKTOP_INIT`:     initDesktop,
	`DESKTOP_PING`:     pingDesktop,
	`DESKTOP_KILL`:     killDesktop,
	`DESKTOP_SHOT`:     getDesktop,
	`COMMAND_EXEC`:     execCommand,
}

/*
目的: サーバーに対して、クライアントがオンラインであることを示すために利用されます。また、クライアントの一部の情報（CPU使用率など）をサーバーに送信します。
動作: GetPartialInfo() 関数でクライアントの基本情報を取得し、サーバーに送信します。
*/
func ping(pack modules.Packet, wsConn *common.Conn) {
	wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	device, err := GetPartialInfo()
	if err != nil {
		golog.Error(err)
		return
	}
	wsConn.SendPack(modules.CommonPack{Act: `DEVICE_UPDATE`, Data: *device})
}

/*
目的: クライアントをオフラインにするために使用されます。
動作: クライアントは自身のWebSocket接続を閉じ、システムを終了します（os.Exit(0)）。
*/
func offline(pack modules.Packet, wsConn *common.Conn) {
	wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	stop = true
	wsConn.Close()
	os.Exit(0)
}

/*
目的: クライアントの画面をロックします（ユーザーがシステムにアクセスできない状態にする）。
動作: basic.Lock() を呼び出してシステムをロックします。成功すればサーバーに成功メッセージを返します。
*/
func lock(pack modules.Packet, wsConn *common.Conn) {
	err := basic.Lock()
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	}
}

/*
目的: クライアントユーザーをログオフさせます。
動作: basic.Logoff() を呼び出してユーザーをログオフさせます。
*/
func logoff(pack modules.Packet, wsConn *common.Conn) {
	err := basic.Logoff()
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	}
}

/*
hibernate/suspend
目的: クライアントのPCをハイバネートまたはスリープ状態にします。
動作: それぞれ basic.Hibernate() や basic.Suspend() を呼び出して実行します。
*/
func hibernate(pack modules.Packet, wsConn *common.Conn) {
	err := basic.Hibernate()
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	}
}

func suspend(pack modules.Packet, wsConn *common.Conn) {
	err := basic.Suspend()
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	}
}

/*
restart/shutdown
目的: クライアントのPCを再起動またはシャットダウンします。
動作: basic.Restart() または basic.Shutdown() を呼び出して実行します。
*/
func restart(pack modules.Packet, wsConn *common.Conn) {
	err := basic.Restart()
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	}
}

func shutdown(pack modules.Packet, wsConn *common.Conn) {
	err := basic.Shutdown()
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	}
}

/*
目的: クライアントのスクリーンショットを取得し、サーバーに送信します。
動作: Screenshot.GetScreenshot() を呼び出し、スクリーンショットを取得して、指定された bridge（通信チャネル）を通してサーバーに送信します。
*/
func screenshot(pack modules.Packet, wsConn *common.Conn) {
	var bridge string
	if val, ok := pack.GetData(`bridge`, reflect.String); !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`}, pack)
		return
	} else {
		bridge = val.(string)
	}
	err := Screenshot.GetScreenshot(bridge)
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	}
}

func initTerminal(pack modules.Packet, wsConn *common.Conn) {
	err := terminal.InitTerminal(pack)
	if err != nil {
		wsConn.SendCallback(modules.Packet{Act: `TERMINAL_INIT`, Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Act: `TERMINAL_INIT`, Code: 0}, pack)
	}
}

func inputTerminal(pack modules.Packet, wsConn *common.Conn) {
	terminal.InputTerminal(pack)
}

func resizeTerminal(pack modules.Packet, wsConn *common.Conn) {
	terminal.ResizeTerminal(pack)
}

func pingTerminal(pack modules.Packet, wsConn *common.Conn) {
	terminal.PingTerminal(pack)
}

func killTerminal(pack modules.Packet, wsConn *common.Conn) {
	terminal.KillTerminal(pack)
}

/*
目的: クライアント上のファイルの一覧を取得したり、ファイルをサーバーに送信します。
動作:
listFiles: 指定されたパスのファイルをリスト化しサーバーに送信します。
fetchFile: 指定されたファイルを取得し、サーバーに送信します。
*/
func listFiles(pack modules.Packet, wsConn *common.Conn) {
	path := `/`
	if val, ok := pack.GetData(`path`, reflect.String); ok {
		path = val.(string)
	}
	files, err := file.ListFiles(path)
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0, Data: smap{`files`: files}}, pack)
	}
}

func fetchFile(pack modules.Packet, wsConn *common.Conn) {
	var path, filename, bridge string
	if val, ok := pack.GetData(`path`, reflect.String); !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|EXPLORER.FILE_OR_DIR_NOT_EXIST}`}, pack)
		return
	} else {
		path = val.(string)
	}
	if val, ok := pack.GetData(`file`, reflect.String); !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`}, pack)
		return
	} else {
		filename = val.(string)
	}
	if val, ok := pack.GetData(`bridge`, reflect.String); !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`}, pack)
		return
	} else {
		bridge = val.(string)
	}
	err := file.FetchFile(path, filename, bridge)
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	}
}

func removeFiles(pack modules.Packet, wsConn *common.Conn) {
	var files []string
	if val, ok := pack.Data[`files`]; !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|EXPLORER.FILE_OR_DIR_NOT_EXIST}`}, pack)
		return
	} else {
		slice := val.([]any)
		for i := 0; i < len(slice); i++ {
			file, ok := slice[i].(string)
			if ok {
				files = append(files, file)
			}
		}
		if len(files) == 0 {
			wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|EXPLORER.FILE_OR_DIR_NOT_EXIST}`}, pack)
			return
		}
	}
	err := file.RemoveFiles(files)
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	}
}

/*
目的: サーバーからクライアントにファイルをアップロードします。
動作:
uploadFiles: ファイルを指定された範囲でアップロードします。
uploadTextFile: テキストファイルをアップロードします。
*/
func uploadFiles(pack modules.Packet, wsConn *common.Conn) {
	var (
		start, end int64
		files      []string
		bridge     string
	)
	if val, ok := pack.Data[`files`]; !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|EXPLORER.FILE_OR_DIR_NOT_EXIST}`}, pack)
		return
	} else {
		slice := val.([]any)
		for i := 0; i < len(slice); i++ {
			file, ok := slice[i].(string)
			if ok {
				files = append(files, file)
			}
		}
		if len(files) == 0 {
			wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|EXPLORER.FILE_OR_DIR_NOT_EXIST}`}, pack)
			return
		}
	}
	if val, ok := pack.GetData(`bridge`, reflect.String); !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`}, pack)
		return
	} else {
		bridge = val.(string)
	}
	{
		if val, ok := pack.GetData(`start`, reflect.Float64); ok {
			start = int64(val.(float64))
		}
		if val, ok := pack.GetData(`end`, reflect.Float64); ok {
			end = int64(val.(float64))
			if end > 0 {
				end++
			}
		}
		if end > 0 && end < start {
			wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`}, pack)
			return
		}
	}
	err := file.UploadFiles(files, bridge, start, end)
	if err != nil {
		golog.Error(err)
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	}
}

func uploadTextFile(pack modules.Packet, wsConn *common.Conn) {
	var path, bridge string
	if val, ok := pack.GetData(`file`, reflect.String); !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|EXPLORER.FILE_OR_DIR_NOT_EXIST}`}, pack)
		return
	} else {
		path = val.(string)
	}
	if val, ok := pack.GetData(`bridge`, reflect.String); !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`}, pack)
		return
	} else {
		bridge = val.(string)
	}
	err := file.UploadTextFile(path, bridge)
	if err != nil {
		golog.Error(err)
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	}
}

/*
目的: クライアント上で実行中のプロセスを一覧表示したり、指定したプロセスを終了します。
動作:
listProcesses: 実行中のプロセスのリストを取得し、サーバーに送信します。
killProcess: 指定されたPIDのプロセスを終了します。
*/
func listProcesses(pack modules.Packet, wsConn *common.Conn) {
	processes, err := process.ListProcesses()
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0, Data: map[string]any{`processes`: processes}}, pack)
	}
}

func killProcess(pack modules.Packet, wsConn *common.Conn) {
	var (
		pid int32
		err error
	)
	if val, ok := pack.GetData(`pid`, reflect.Float64); !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`}, pack)
		return
	} else {
		pid = int32(val.(float64))
	}
	err = process.KillProcess(int32(pid))
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0}, pack)
	}
}

/*
目的: デスクトップ共有またはリモート操作を実行します。
動作:
initDesktop: デスクトップセッションを開始します。
pingDesktop: デスクトップセッションの状態を確認します。
killDesktop: デスクトップセッションを終了します。
getDesktop: デスクトップのスクリーンショットを取得します。
*/
func initDesktop(pack modules.Packet, wsConn *common.Conn) {
	err := desktop.InitDesktop(pack)
	if err != nil {
		wsConn.SendCallback(modules.Packet{Act: `DESKTOP_INIT`, Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Act: `DESKTOP_INIT`, Code: 0}, pack)
	}
}

func pingDesktop(pack modules.Packet, wsConn *common.Conn) {
	desktop.PingDesktop(pack)
}

func killDesktop(pack modules.Packet, wsConn *common.Conn) {
	desktop.KillDesktop(pack)
}

func getDesktop(pack modules.Packet, wsConn *common.Conn) {
	desktop.GetDesktop(pack)
}

/*
目的: クライアント側でコマンドを実行します。
動作: サーバーから指定されたコマンド（および引数）を実行し、その結果をサーバーに返します。
*/
func execCommand(pack modules.Packet, wsConn *common.Conn) {
	var proc *exec.Cmd
	var cmd, args string
	if val, ok := pack.Data[`cmd`]; !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`}, pack)
		return
	} else {
		cmd = val.(string)
	}
	if val, ok := pack.Data[`args`]; !ok {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`}, pack)
		return
	} else {
		args = val.(string)
	}
	if len(args) == 0 {
		proc = exec.Command(cmd)
	} else {
		proc = exec.Command(cmd, strings.Split(args, ` `)...)
	}
	err := proc.Start()
	if err != nil {
		wsConn.SendCallback(modules.Packet{Code: 1, Msg: err.Error()}, pack)
	} else {
		wsConn.SendCallback(modules.Packet{Code: 0, Data: map[string]any{
			`pid`: proc.Process.Pid,
		}}, pack)
		proc.Process.Release()
	}
}

func inputRawTerminal(pack []byte, event string) {
	terminal.InputRawTerminal(pack, event)
}
