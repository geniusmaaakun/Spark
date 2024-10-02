package main

import (
	"Spark/client/config"
	"Spark/client/core"
	"Spark/utils"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kataras/golog"
)

/*
Spark クライアントアプリケーションの起動と設定ファイルの読み込み、更新処理、暗号化されたデータの復号処理を行うプログラムです。以下に、各部分を説明します。
*/

/*
1. 初期化 (init 関数)
init 関数は、プログラムの実行時に自動的に呼び出されます。ここでは、ログのタイムフォーマットを設定し、暗号化された設定データ (ConfigBuffer) を復号化して構成情報を読み込みます。

golog.SetTimeFormat は、ログのタイムスタンプフォーマットを設定しています。
config.ConfigBuffer が暗号化された設定データを保持しています。もしデータが空であれば (\x19 で埋められている場合)、プログラムを終了します。
ConfigBuffer の先頭2バイトを数値に変換し、それをデータ長として使用します。これにより、設定データの長さを決定します。
暗号化された設定データの最初の16バイトを復号キーとして使用し、それ以降のデータを復号化します。復号されたデータは config.Config に保存されます。
最後に、config.Config.Path がスラッシュ (/) で終わっている場合、そのスラッシュを削除します。
*/
func init() {
	golog.SetTimeFormat(`2006/01/02 15:04:05`)

	if len(strings.Trim(config.ConfigBuffer, "\x19")) == 0 {
		os.Exit(0)
		return
	}

	// Convert first 2 bytes to int, which is the length of the encrypted config.
	dataLen := int(big.NewInt(0).SetBytes([]byte(config.ConfigBuffer[:2])).Uint64())
	if dataLen > len(config.ConfigBuffer)-2 {
		os.Exit(1)
		return
	}
	cfgBytes := utils.StringToBytes(config.ConfigBuffer, 2, 2+dataLen)
	cfgBytes, err := decrypt(cfgBytes[16:], cfgBytes[:16])
	if err != nil {
		os.Exit(1)
		return
	}
	err = utils.JSON.Unmarshal(cfgBytes, &config.Config)
	if err != nil {
		os.Exit(1)
		return
	}
	if strings.HasSuffix(config.Config.Path, `/`) {
		config.Config.Path = config.Config.Path[:len(config.Config.Path)-1]
	}
}

/*
main 関数は、クライアントプログラムのエントリポイントです。

update() 関数を呼び出して、更新処理を行います。
core.Start() を呼び出して、クライアントのメイン機能を開始します。
*/
func main() {
	update()
	core.Start()
}

/*
この関数は、クライアントの自己更新を行います。

プログラムの実行パスを取得し、--update 引数が渡されている場合は、自分自身のバイナリをコピーして更新を試みます。
更新が成功した場合は終了します。
--clean 引数が渡されている場合は、3秒後に一時ファイルを削除します。
*/
func update() {
	/*
		os.Executable() は、実行中のプログラムの絶対パスを返します。
		例えば、/usr/local/bin/myprogram のような形式です。
		この呼び出しがエラーを返した場合、os.Args[0]（コマンドラインで指定されたプログラムのパス）を使います。
		os.Args[0] はプログラムの名前を指しますが、相対パスや簡易な名前だけの場合もあるため、os.Executable() の方が信頼性が高いです。
	*/
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = os.Args[0]
	}
	/*
		core.start()で更新されるプログラムは、一時ファイルとして保存されます。この一時ファイルは、元のプログラムのパスに .tmp 拡張子が付いたものです。


		この部分では、プログラムが --update 引数付きで実行されたかを確認します。
		os.Args[1] == --update は、コマンドライン引数に --update が含まれている場合を意味します。
		その後、selfPath の長さを確認します。これは、selfPath が実行可能なファイルパスであることを確かめるための簡易なチェックです。
		長さが4バイト以下（例えば、/bin など）なら処理を中断します。
	*/
	if len(os.Args) > 1 && os.Args[1] == `--update` {
		if len(selfPath) <= 4 {
			return
		}
		/*
			destPath := selfPath[:len(selfPath)-4]:

			この部分では、ファイルパスの最後の4文字を取り除き、新しいファイルパスを生成しています。
			例えば、selfPath が /usr/local/bin/myprogram.tmp の場合、destPath は /usr/local/bin/myprogram となります。この動作は、一時ファイル（*.tmp）からオリジナルのファイルを復元するために行います。
			os.ReadFile(selfPath):

			現在実行中のプログラム（selfPath）の内容を読み込みます。
			os.WriteFile(destPath, thisFile, 0755):

			読み込んだプログラムの内容を destPath に書き込み、元のプログラムファイルに上書きします。ファイルモード 0755 は、プログラムが実行可能であることを示します。
			exec.Command(destPath, --clean):

			新しいバイナリ（destPath）を --clean 引数付きで実行します。この処理は、新しいプログラムが一時ファイルを削除するために行われます。
			cmd.Start():

			新しいプロセスを非同期で開始します。エラーがなければ、os.Exit(0) で現在のプログラムを終了し、新しいプロセスに制御を引き渡します。
		*/
		destPath := selfPath[:len(selfPath)-4]
		thisFile, err := os.ReadFile(selfPath)
		if err != nil {
			return
		}
		os.WriteFile(destPath, thisFile, 0755)
		cmd := exec.Command(destPath, `--clean`)
		if cmd.Start() == nil {
			os.Exit(0)
			return
		}
	}
	/*
		プログラムが --clean 引数で実行された場合、クリーンアップ処理が行われます。
		まず、3秒待機するために time.After を使って一時的なスリープを行います。この待機は、新しいプログラムの更新処理が確実に完了するのを待つためです。
		その後、os.Remove(selfPath + .tmp) で、一時ファイルを削除します。
		selfPath + ".tmp" は、.tmp 拡張子がついた一時ファイルの名前です。os.Remove はこのファイルを削除します。
	*/
	if len(os.Args) > 1 && os.Args[1] == `--clean` {
		<-time.After(3 * time.Second)
		os.Remove(selfPath + `.tmp`)
	}
}

/*
この関数は、暗号化された設定データを復号化するために使用されます。

data の最初の16バイトはMD5ハッシュであり、残りのデータをAES-CTRモードで復号化します。
復号化後、データの整合性を確認するために、元のハッシュと復号されたデータのMD5ハッシュを比較します。一致しない場合はエラーを返します。
*/
func decrypt(data []byte, key []byte) ([]byte, error) {
	// MD5[16 bytes] + Data[n bytes]
	dataLen := len(data)
	if dataLen <= 16 {
		return nil, utils.ErrEntityInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(block, data[:16])
	decBuffer := make([]byte, dataLen-16)
	stream.XORKeyStream(decBuffer, data[16:])
	hash, _ := utils.GetMD5(decBuffer)
	if !bytes.Equal(hash, data[:16]) {
		return nil, utils.ErrFailedVerification
	}
	return decBuffer[:dataLen-16], nil
}
