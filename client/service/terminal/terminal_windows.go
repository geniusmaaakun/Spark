package terminal

import (
	"Spark/client/common"
	"Spark/modules"
	"Spark/utils"
	"Spark/utils/cmap"
	"encoding/hex"
	"io"
	"os/exec"
	"reflect"
	"syscall"
	"time"
)

/*
Windows環境で仮想端末（Terminal）セッションを管理し、リモートからコマンドライン操作を行うための実装です。ユーザーがリモートからコマンドを送信し、その結果を受信することができるようになっています。


このコードは、Windowsシステムにおいて仮想端末セッションを管理し、リモートクライアントとの間でコマンドやその出力をやり取りするためのものです。標準出力・エラー出力のデータをリモートに送信し、リモートからの入力を処理します。また、ヘルスチェックにより、非アクティブなセッションを自動的に終了します。
*/

/*
仮想端末セッションの情報を管理します。
lastPack: 最後にパケットが受信された時間（UNIXタイム）。
rawEvent: セッションのイベントIDをバイナリデータで保持。
escape: セッションが終了状態かどうかを管理するフラグ。
event: イベントID。
cmd: 実行中のコマンド（exec.Cmd）。
stdout, stderr, stdin: 標準出力、標準エラー出力、標準入力のハンドル。
*/
type terminal struct {
	lastPack int64
	rawEvent []byte
	escape   bool
	event    string
	cmd      *exec.Cmd
	stdout   *io.ReadCloser
	stderr   *io.ReadCloser
	stdin    *io.WriteCloser
}

var terminals = cmap.New[*terminal]()
var defaultCmd = ``

/*
初期化処理。WindowsのコンソールエンコーディングをUTF-8に設定します。
SetConsoleCP と SetConsoleOutputCP を使用して、コンソールの入力・出力をUTF-8に変更します。
端末セッションのヘルスチェックを定期的に行う healthCheck ゴルーチンを開始します。
*/
func init() {
	defer func() {
		recover()
	}()
	{
		kernel32 := syscall.NewLazyDLL(`kernel32.dll`)
		kernel32.NewProc(`SetConsoleCP`).Call(65001)
		kernel32.NewProc(`SetConsoleOutputCP`).Call(65001)
	}
	go healthCheck()
}

/*
仮想端末セッションを初期化します。
cmd に指定されたターミナル（powershell.exe または cmd.exe）を起動し、標準入出力を設定します。
ターミナルのセッションを管理するために、各セッションごとに readSender ゴルーチンを実行し、標準出力とエラー出力を読み取ります。
出力が1KB以上であればバイナリデータとして、1KB以下であればJSONとしてリモートクライアントに送信します。
*/
func InitTerminal(pack modules.Packet) error {
	cmd := exec.Command(getTerminal())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	rawEvent, _ := hex.DecodeString(pack.Event)
	session := &terminal{
		cmd:      cmd,
		event:    pack.Event,
		escape:   false,
		stdout:   &stdout,
		stderr:   &stderr,
		stdin:    &stdin,
		rawEvent: rawEvent,
		lastPack: utils.Unix,
	}

	readSender := func(rc io.ReadCloser) {
		bufSize := 1024
		for !session.escape {
			buffer := make([]byte, bufSize)
			n, err := rc.Read(buffer)
			buffer = buffer[:n]

			// if output is larger than 1KB, then send binary data
			if n > 1024 {
				if bufSize < 32768 {
					bufSize *= 2
				}
				common.WSConn.SendRawData(session.rawEvent, buffer, 21, 00)
			} else {
				bufSize = 1024
				buffer, _ = utils.JSON.Marshal(modules.Packet{Act: `TERMINAL_OUTPUT`, Data: map[string]any{
					`output`: hex.EncodeToString(buffer),
				}})
				buffer = utils.XOR(buffer, common.WSConn.GetSecret())
				common.WSConn.SendRawData(session.rawEvent, buffer, 21, 01)
			}

			session.lastPack = utils.Unix
			if err != nil {
				if !session.escape {
					session.escape = true
					doKillTerminal(session)
				}
				data, _ := utils.JSON.Marshal(modules.Packet{Act: `TERMINAL_QUIT`})
				data = utils.XOR(data, common.WSConn.GetSecret())
				common.WSConn.SendRawData(session.rawEvent, data, 21, 01)
				break
			}
		}
	}
	go readSender(stdout)
	go readSender(stderr)

	err = cmd.Start()
	if err != nil {
		session.escape = true
		return err
	}
	terminals.Set(pack.Data[`terminal`].(string), session)
	return nil
}

func InputRawTerminal(input []byte, uuid string) {
	session, ok := terminals.Get(uuid)
	if !ok {
		return
	}
	(*session.stdin).Write(input)
	session.lastPack = utils.Unix
}

/*
リモートクライアントから送信された入力を受け取り、対応する端末セッションに書き込みます。
入力は hex.DecodeString を用いてデコードされ、仮想端末の stdin に送信されます。
*/
func InputTerminal(pack modules.Packet) {
	var err error
	var uuid string
	var input []byte
	var session *terminal

	if val, ok := pack.GetData(`input`, reflect.String); !ok {
		return
	} else {
		if input, err = hex.DecodeString(val.(string)); err != nil {
			return
		}
	}
	if val, ok := pack.GetData(`terminal`, reflect.String); !ok {
		return
	} else {
		uuid = val.(string)
		if val, ok = terminals.Get(uuid); ok {
			session = val.(*terminal)
		} else {
			return
		}
	}
	(*session.stdin).Write(input)
	session.lastPack = utils.Unix
}

/*
仮想端末のリサイズ処理。Windowsではこの機能はサポートされていないため、実装されていません（常に nil を返します）。
*/
func ResizeTerminal(pack modules.Packet) error {
	return nil
}

/*
指定された仮想端末セッションを終了します。
セッションのリソースを解放し、終了メッセージをリモートクライアントに送信します。
*/
func KillTerminal(pack modules.Packet) {
	var uuid string
	if val, ok := pack.GetData(`terminal`, reflect.String); !ok {
		return
	} else {
		uuid = val.(string)
	}
	session, ok := terminals.Get(uuid)
	if !ok {
		return
	}
	terminals.Remove(uuid)
	data, _ := utils.JSON.Marshal(modules.Packet{Act: `TERMINAL_QUIT`, Msg: `${i18n|TERMINAL.SESSION_CLOSED}`})
	data = utils.XOR(data, common.WSConn.GetSecret())
	common.WSConn.SendRawData(session.rawEvent, data, 21, 01)
	session.escape = true
	session.rawEvent = nil
	doKillTerminal(session)
}

/*
端末セッションがまだアクティブかどうかを確認します。リモートからの "ping" リクエストを処理し、セッションの lastPack タイムスタンプを更新します。
*/
func PingTerminal(pack modules.Packet) {
	var uuid string
	var session *terminal
	if val, ok := pack.GetData(`terminal`, reflect.String); !ok {
		return
	} else {
		uuid = val.(string)
	}
	session, ok := terminals.Get(uuid)
	if !ok {
		return
	}
	session.lastPack = utils.Unix
}

/*
仮想端末セッションを強制的に終了します。
標準入出力を閉じ、プロセスを終了させます。
*/
func doKillTerminal(terminal *terminal) {
	(*terminal.stdout).Close()
	(*terminal.stderr).Close()
	(*terminal.stdin).Close()
	if terminal.cmd.Process != nil {
		terminal.cmd.Process.Kill()
		terminal.cmd.Process.Wait()
		terminal.cmd.Process.Release()
	}
}

/*
使用可能なターミナル（powershell.exe または cmd.exe）を検出し、デフォルトのターミナルを設定します。
*/
func getTerminal() string {
	var cmdTable = []string{
		`powershell.exe`,
		`cmd.exe`,
	}
	if defaultCmd != `` {
		return defaultCmd
	}
	for _, cmd := range cmdTable {
		if _, err := exec.LookPath(cmd); err == nil {
			defaultCmd = cmd
			return cmd
		}
	}
	return `cmd.exe`
}

/*
定期的に仮想端末セッションのヘルスチェックを行います。
セッションが一定時間（300秒）アクティブでなかった場合、自動的にセッションを終了します。
*/
func healthCheck() {
	const MaxInterval = 300
	for now := range time.NewTicker(30 * time.Second).C {
		timestamp := now.Unix()
		// stores sessions to be disconnected
		keys := make([]string, 0)
		terminals.IterCb(func(uuid string, session *terminal) bool {
			if timestamp-session.lastPack > MaxInterval {
				keys = append(keys, uuid)
				doKillTerminal(session)
			}
			return true
		})
		terminals.Remove(keys...)
	}
}
