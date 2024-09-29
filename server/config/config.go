package config

import (
	"Spark/utils"
	"bytes"
	"flag"
	"os"

	"github.com/kataras/golog"
)

/*
Go言語を使用して構成ファイル（config.json）からサーバーの設定を読み込み、その設定に基づいてサーバーの動作を制御する設定管理システムです。
さらに、コマンドライン引数を処理し、設定を上書きする機能も提供しています。


このコードは、サーバーの設定を管理するための初期化処理を提供しています。主な機能としては以下が含まれます。

設定ファイルの読み込み: config.jsonやConfig.jsonファイルを読み込み、設定値をConfig構造体に格納します。
コマンドライン引数の処理: 設定ファイルの値をコマンドライン引数で上書き可能です。
ログ管理: ログレベル、ログパス、ログ保持期間などの設定を管理します。
ソルト管理: サーバーのソルト（暗号化用のキー）を24バイトに調整して設定します。
これにより、サーバーが適切な設定で動作するように準備し、エラーハンドリングも適切に行う仕組みを提供しています。
*/

/*
**config**構造体は、サーバーの設定を保持します。

Listen: サーバーの待ち受けアドレス。デフォルトは:8000で、localhost:8000で待ち受ける設定です。
Salt: サーバーで使用するソルト（暗号化キーの一部）。
Auth: 認証情報（ユーザー名とパスワードのペア）を保持するマップです。
Log: ログ関連の設定（ログレベル、ログパス、ログの保存期間）を保持するlog構造体。
SaltBytes: Saltのバイト表現です。内部的に暗号化に使用されますが、json:"-"により、JSONにシリアライズされません。
*/
type config struct {
	Listen    string            `json:"listen"`
	Salt      string            `json:"salt"`
	Auth      map[string]string `json:"auth"`
	Log       *log              `json:"log"`
	SaltBytes []byte            `json:"-"`
}

/*
**log**構造体はログの設定を保持します。

Level: ログレベル（例：info、debug、errorなど）。
Path: ログファイルの保存パス。
Days: ログファイルの保持期間（日数）。
*/
type log struct {
	Level string `json:"level"`
	Path  string `json:"path"`
	Days  uint   `json:"days"`
}

/*
COMMIT: 現在のビルドのコミットハッシュを保持する変数（自動アップグレード用の情報として使用される可能性があります）。
Config: 設定情報を保持するconfig構造体のインスタンス。
BuiltPath: ビルドされたファイルのパス（フォーマットを使って構築されるパスのテンプレート）。
*/
// COMMIT is hash of this commit, for auto upgrade.
var COMMIT = ``
var Config config
var BuiltPath = `./built/%v_%v`

/*
init関数は、パッケージが初期化されると自動的に呼び出されます。ここでは以下の処理を行います。

golog.SetTimeFormat: ログのタイムフォーマットを設定します。

*/
func init() {
	golog.SetTimeFormat(`2006/01/02 15:04:05`)

	var (
		err                      error
		configData               []byte
		configPath, listen, salt string
		username, password       string
		logLevel, logPath        string
		logDays                  uint
	)
	//コマンドライン引数を使用して設定を上書きできるようにしています。例として、ログレベルやサーバーのリッスンアドレス、ユーザー名、パスワードなどがコマンドライン引数から指定できます。
	flag.StringVar(&configPath, `config`, `config.json`, `config file path, default: config.json`)
	flag.StringVar(&listen, `listen`, `:8000`, `required, listen address, default: :8000`)
	flag.StringVar(&salt, `salt`, ``, `required, salt of server`)
	flag.StringVar(&username, `username`, ``, `username of web interface`)
	flag.StringVar(&password, `password`, ``, `password of web interface`)
	flag.StringVar(&logLevel, `log-level`, `info`, `log level, default: info`)
	flag.StringVar(&logPath, `log-path`, `./logs`, `log file path, default: ./logs`)
	flag.UintVar(&logDays, `log-days`, 7, `max days of logs, default: 7`)
	flag.Parse()

	if len(configPath) > 0 {
		configData, err = os.ReadFile(configPath)
		if err != nil {
			configData, err = os.ReadFile(`Config.json`)
			if err != nil {
				fatal(map[string]any{
					`event`:  `CONFIG_LOAD`,
					`status`: `fail`,
					`msg`:    err.Error(),
				})
				return
			}
		}
		//設定ファイルがconfig.jsonから読み込まれます。ファイルが見つからない場合、デフォルトのConfig.jsonが試され、それでも失敗すればエラーログを出力して終了します。
		err = utils.JSON.Unmarshal(configData, &Config)
		if err != nil {
			fatal(map[string]any{
				`event`:  `CONFIG_PARSE`,
				`status`: `fail`,
				`msg`:    err.Error(),
			})
			return
		}
		if Config.Log == nil {
			Config.Log = &log{
				Level: `info`,
				Path:  `./logs`,
				Days:  7,
			}
		}
	} else {
		Config = config{
			Listen: listen,
			Salt:   salt,
			Auth: map[string]string{
				username: password,
			},
			Log: &log{
				Level: logLevel,
				Path:  logPath,
				Days:  logDays,
			},
		}
	}

	//ソルトの長さが24バイト以下であるか確認します。24バイト以上の場合、エラーメッセージを出力して終了します。
	if len(Config.Salt) > 24 {
		fatal(map[string]any{
			`event`:  `CONFIG_PARSE`,
			`status`: `fail`,
			`msg`:    `length of salt should less than 24`,
		})
		return
	}
	//ソルトが24バイトに満たない場合、25というバイト値で埋めて24バイトに調整します。
	Config.SaltBytes = []byte(Config.Salt)
	Config.SaltBytes = append(Config.SaltBytes, bytes.Repeat([]byte{25}, 24)...)
	Config.SaltBytes = Config.SaltBytes[:24]

	golog.SetLevel(utils.If(len(Config.Log.Level) == 0, `info`, Config.Log.Level))
}

//fatal関数は、致命的なエラーが発生した際にエラーメッセージをJSON形式で生成し、golog.Fatalを使って出力します。出力後、プログラムは終了します。
func fatal(args map[string]any) {
	output, _ := utils.JSON.MarshalToString(args)
	golog.Fatal(output)
}
