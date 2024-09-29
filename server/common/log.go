package common

import (
	"Spark/server/config"
	"Spark/utils"
	"Spark/utils/melody"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kataras/golog"
)

/*
Go言語を使用してログ管理を行うためのパッケージです。
gologライブラリを用いて、ログの出力先、ログレベルの設定、タイムスタンプ付きのログファイルの作成、そしてコンテキストに基づいた詳細なログ情報の生成を行っています。また、ginやmelodyセッションの情報を使って、クライアントのIPアドレスやデバイス情報をログに記録しています。


このコードは、次のような機能を持つログ管理システムを実装しています。

ログのファイル出力: 毎日ログファイルを切り替え、ログを指定のディレクトリに保存します。古いログファイルは自動的に削除されます。
ログの出力形式: ログはJSON形式で出力され、リクエストに関するコンテキスト情報（IPアドレス、セッション情報など）も含まれます。
ログレベル: Info、Warn、Error、Fatal、Debugの各レベルでログを出力できます。
安全なシャットダウン: CloseLogでログシステムを安全に終了し、必要に応じてログの出力先を標準出力に戻します。
これにより、アプリケーションの動作状況を詳細に記録し、障害発生時やデバッグ時に役立つログを効率的に管理できます。
*/

// logWriter: 現在使用中のログファイルへの書き込みストリームを保持するファイルポインタ。
// disposed: ログシステムが停止状態かどうかを管理するフラグ。ログシステムが終了していればtrueになります。
var logWriter *os.File
var disposed bool

/*
init関数はパッケージが初期化されたときに自動的に実行され、ログの設定と出力先を決定します。
setLogDst関数:
ログの出力先を設定します。config.Config.Log.Pathで指定されたディレクトリにログファイルを作成し、毎日新しいログファイルに切り替えます。
ログが無効化されている場合やシステムが終了状態の場合は、標準出力（os.Stdout）にログを出力します。
古いログファイルは、設定で指定された日数（config.Config.Log.Days）以上経過すると自動的に削除されます。
定期的なログファイルのローテーション:
初回実行後、毎日午前0時にログファイルが新しくなります。
*/
func init() {
	setLogDst := func() {
		var err error
		if logWriter != nil {
			logWriter.Close()
		}
		if config.Config.Log.Level == `disable` || disposed {
			golog.SetOutput(os.Stdout)
			return
		}
		os.Mkdir(config.Config.Log.Path, 0666)
		now := utils.Now.Add(time.Minute)
		logFile := fmt.Sprintf(`%s/%s.log`, config.Config.Log.Path, now.Format(`2006-01-02`))
		logWriter, err = os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			golog.Warn(getLog(nil, `LOG_INIT`, `fail`, err.Error(), nil))
		}
		golog.SetOutput(io.MultiWriter(os.Stdout, logWriter))

		staleDate := time.Unix(now.Unix()-int64(config.Config.Log.Days*86400), 0)
		staleLog := fmt.Sprintf(`%s/%s.log`, config.Config.Log.Path, staleDate.Format(`2006-01-02`))
		os.Remove(staleLog)
	}
	setLogDst()
	go func() {
		waitSecs := 86400 - (utils.Now.Hour()*3600 + utils.Now.Minute()*60 + utils.Now.Second())
		if waitSecs > 0 {
			<-time.After(time.Duration(waitSecs) * time.Second)
		}
		setLogDst()
		for range time.NewTicker(time.Second * 86400).C {
			setLogDst()
		}
	}()
}

/*
この関数は、与えられたコンテキスト（ctx）、イベント名（event）、ステータス（status）、メッセージ（msg）を基にログメッセージを生成します。
ctx: Ginの*gin.Contextやmelody.Sessionなど、リクエストのコンテキストやセッションに基づいて、クライアントのIPアドレスやデバイスの詳細情報を取得します。
args: ログに含める追加の情報を保持するマップです。eventやstatusなどの情報もマップに格納されます。
出力例: ログメッセージは最終的にJSON形式で出力されます。utils.JSON.MarshalToStringによって、マップargsがJSON文字列に変換されます。
*/
func getLog(ctx any, event, status, msg string, args map[string]any) string {
	if args == nil {
		args = map[string]any{}
	}
	args[`event`] = event
	if len(msg) > 0 {
		args[`msg`] = msg
	}
	if len(status) > 0 {
		args[`status`] = status
	}
	if ctx != nil {
		var connUUID string
		var targetInfo bool
		switch ctx.(type) {
		case *gin.Context:
			c := ctx.(*gin.Context)
			args[`from`] = GetRealIP(c)
			connUUID, targetInfo = c.Request.Context().Value(`ConnUUID`).(string)
		case *melody.Session:
			s := ctx.(*melody.Session)
			args[`from`] = GetAddrIP(s.GetWSConn().UnderlyingConn().RemoteAddr())
			if deviceConn, ok := args[`deviceConn`]; ok {
				delete(args, `deviceConn`)
				connUUID = deviceConn.(*melody.Session).UUID
				targetInfo = true
			}
		}
		if targetInfo {
			device, ok := Devices.Get(connUUID)
			if ok {
				args[`target`] = map[string]any{
					`name`: device.Hostname,
					`ip`:   device.WAN,
				}
			}
		}
	}
	output, _ := utils.JSON.MarshalToString(args)
	return output
}

/*
これらの関数は、getLog関数を使って生成されたログメッセージを、gologライブラリの対応するログレベル（Info、Warn、Error、Fatal、Debug）で出力します。
Info: 情報レベルのログ。
Warn: 警告レベルのログ。
Error: エラーレベルのログ。
Fatal: 致命的なエラーで、ログ出力後にプログラムが終了します。
Debug: デバッグ用のログ。
*/
func Info(ctx any, event, status, msg string, args map[string]any) {
	golog.Infof(getLog(ctx, event, status, msg, args))
}

func Warn(ctx any, event, status, msg string, args map[string]any) {
	golog.Warnf(getLog(ctx, event, status, msg, args))
}

func Error(ctx any, event, status, msg string, args map[string]any) {
	golog.Error(getLog(ctx, event, status, msg, args))
}

func Fatal(ctx any, event, status, msg string, args map[string]any) {
	golog.Fatalf(getLog(ctx, event, status, msg, args))
}

func Debug(ctx any, event, status, msg string, args map[string]any) {
	golog.Debugf(getLog(ctx, event, status, msg, args))
}

//**CloseLog**は、ログシステムを終了し、ログの出力先を標準出力（os.Stdout）に戻します。また、現在のログファイルが開かれている場合は、それをクローズします。
func CloseLog() {
	disposed = true
	golog.SetOutput(os.Stdout)
	if logWriter != nil {
		logWriter.Close()
		logWriter = nil
	}
}
