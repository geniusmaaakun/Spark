package generate

import (
	"Spark/modules"
	"Spark/server/common"
	"Spark/server/config"
	"Spark/utils"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

/*
クライアント側のバイナリファイル生成および設定ファイルを暗号化して埋め込む処理を提供するWebサーバーAPIを実装しています。
クライアント（OSやアーキテクチャに依存する実行ファイル）に対して設定情報（UUID、キー、ホスト情報など）を埋め込み、そのファイルをダウンロードできるようにします。


リモートクライアントの設定生成とファイルダウンロードを行うWebサーバーの一部を構成しています。設定情報を暗号化し、OSやアーキテクチャに応じたクライアントバイナリに埋め込んで送信する仕組みが実装されています。
*/

//clientCfg 構造体: クライアント側の設定を表現する構造体で、セキュアな接続かどうかやホスト情報、ポート、UUID、暗号キーなどを保持します。
/*
役割: クライアントの接続設定を保持するための構造体です。
Secureがtrueの場合はSSLを使用することを示し、HostやPort、Pathはクライアントが接続するための情報です。
UUIDとKeyはクライアントごとに異なる識別子および暗号化キーとして使用されます。
*/
type clientCfg struct {
	Secure bool   `json:"secure"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Path   string `json:"path"`
	UUID   string `json:"uuid"`
	Key    string `json:"key"`
}

var (
	ErrTooLargeEntity = errors.New(`length of data can not excess buffer size`)
)

//CheckClient 関数: クライアントが存在するかどうか、設定が正しいかを検証します。
/*
役割: リクエストされたOSやアーキテクチャに対応するクライアントバイナリファイルが存在するかを確認します。
クライアント設定の生成も試みますが、設定が大きすぎる場合や生成に失敗した場合は、適切なHTTPエラーを返します。
*/
//クライアントのリクエストを検証し、指定された設定に基づいてクライアントのバイナリファイルが存在するか、設定の生成が可能かを確認するAPIエンドポイントを実装しています。
/*
目的:

リクエストパラメータを検証。
クライアントのバイナリファイルが存在するかを確認。
設定が正しく生成できるかをチェック。

引数:
ctx *gin.Context: Ginフレームワークのコンテキストオブジェクト。

戻り値: なし。適切なHTTPステータスとJSONレスポンスを返します。
*/
func CheckClient(ctx *gin.Context) {
	//リクエストパラメータのバインディングと検証
	//構造体 form を定義し、リクエストパラメータを受け取る。
	var form struct {
		OS     string `json:"os" yaml:"os" form:"os" binding:"required"`
		Arch   string `json:"arch" yaml:"arch" form:"arch" binding:"required"`
		Host   string `json:"host" yaml:"host" form:"host" binding:"required"`
		Port   uint16 `json:"port" yaml:"port" form:"port" binding:"required"`
		Path   string `json:"path" yaml:"path" form:"path" binding:"required"`
		Secure string `json:"secure" yaml:"secure" form:"secure"`
	}
	//パラメータのバインディング（ctx.ShouldBind(&form)）
	//リクエストボディのJSONやフォームデータを form にバインド。
	//必須フィールド（binding:"required"）が欠けている場合はエラー。
	if err := ctx.ShouldBind(&form); err != nil {
		//エラー時の処理:
		//HTTP 400（Bad Request）を返して処理を終了。
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	//クライアントバイナリファイルの存在確認
	//config.BuiltPath:
	// クライアントのバイナリファイルが保存されているディレクトリパス。
	// バイナリファイルの存在確認:
	// 指定された OS とアーキテクチャ（form.OS、form.Arch）に対応するファイルが存在するかを確認。
	_, err := os.Stat(fmt.Sprintf(config.BuiltPath, form.OS, form.Arch))
	// エラー時の処理:
	// ファイルが存在しない場合、HTTP 404（Not Found）を返す。
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.NO_PREBUILT_FOUND}`})
		return
	}
	//設定ファイルの生成チェック
	//genConfig:
	// クライアント設定を暗号化して生成する関数。
	// 入力パラメータ:
	// Secure: HTTPS（true or false）。
	// Host、Port、Path: クライアントが接続するための情報。
	// UUID、Key: プレースホルダー（実際にはクライアントごとに一意の値に置き換えられる）。
	_, err = genConfig(clientCfg{
		Secure: form.Secure == `true`,
		Host:   form.Host,
		Port:   int(form.Port),
		Path:   form.Path,
		UUID:   strings.Repeat(`FF`, 16),
		Key:    strings.Repeat(`FF`, 32),
	})
	//エラー時の処理:
	// 生成された設定が大きすぎる場合:
	if err != nil {
		//HTTP 413（Payload Too Large）を返す。
		if err == ErrTooLargeEntity {
			ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.CONFIG_TOO_LARGE}`})
			return
		}
		//その他
		//HTTP 500（Internal Server Error）を返す。
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.CONFIG_GENERATE_FAILED}`})
		return
	}

	//すべてのチェックが成功した場合、HTTP 200（OK）を返す。
	// modules.Packet{Code: 0}:
	// 成功を示すレスポンス。
	ctx.JSON(http.StatusOK, modules.Packet{Code: 0})

	/*
			動作の流れ
		パラメータ検証:
		必須パラメータが正しい形式で送信されているかを確認。
		バイナリファイルの存在確認:
		指定された OS とアーキテクチャに対応するファイルが存在するかを確認。
		設定ファイルの生成チェック:
		指定された設定に基づいてファイルが正しく生成可能かを確認。
		レスポンス送信:
		エラーがなければ成功レスポンスを返す。
	*/
}

//GenerateClient 関数: クライアントのバイナリファイルを生成し、設定情報を埋め込んだバイナリファイルをレスポンスとして返します。
/*
役割: クライアントの実行ファイル（OSとアーキテクチャに対応したバイナリファイル）を生成し、設定情報を埋め込んでレスポンスとして送信します。
ファイルをバイト単位で読み込み、指定された位置にクライアントの設定情報を暗号化して埋め込みます。
ファイルはストリーミングで送信され、HTTPヘッダーには適切なファイル名やコンテンツ情報が設定されます。
*/
//カスタマイズされたクライアントバイナリを生成し、それをクライアントにダウンロードさせるAPIエンドポイントを実装したものです。主に以下のような機能を提供しています：
func GenerateClient(ctx *gin.Context) {
	//リクエストの検証:
	// クライアントが送信したリクエストのパラメータをチェック。
	var form struct {
		OS     string `json:"os" yaml:"os" form:"os" binding:"required"`
		Arch   string `json:"arch" yaml:"arch" form:"arch" binding:"required"`
		Host   string `json:"host" yaml:"host" form:"host" binding:"required"`
		Port   uint16 `json:"port" yaml:"port" form:"port" binding:"required"`
		Path   string `json:"path" yaml:"path" form:"path" binding:"required"`
		Secure string `json:"secure" yaml:"secure" form:"secure"`
	}
	// リクエストパラメータの検証
	// 必要なパラメータが正しい形式であることを確認。
	if err := ctx.ShouldBind(&form); err != nil {
		//パラメータ（OS、Arch、Host、Port、Pathなど）を構造体 form にバインド。
		// 必須項目が不足している場合は、HTTP 400エラーを返して処理を終了。
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	// templateのバイナリファイルを読み込む
	//OSとアーキテクチャに基づいてテンプレートバイナリを指定されたパスから読み込む。
	// ファイルが存在しない場合は、HTTP 404エラーを返す。
	tpl, err := os.Open(fmt.Sprintf(config.BuiltPath, form.OS, form.Arch))
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.NO_PREBUILT_FOUND}`})
		return
	}
	defer tpl.Close()

	//クライアント設定の生成と埋め込み:
	// クライアント設定（Host、Port、Path、UUIDなど）を暗号化して生成。
	// テンプレート内のプレースホルダー（特定のバイト列）を生成された設定に置き換える。
	clientUUID := utils.GetUUID()
	clientKey, err := common.EncAES(clientUUID, config.Config.SaltBytes)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.CONFIG_GENERATE_FAILED}`})
		return
	}
	/*
		ここで cfgBytes が生成されます。
		genConfig 関数は、clientCfg 構造体を元にクライアントの設定をバイト配列（[]byte）として生成します。この cfgBytes が後でテンプレート内の cfgBuffer と置き換えられます。
		clientCfg には、以下のような情報が含まれます:
		Secure: HTTPS を使用するかどうかを示すフラグ。
		Host: クライアントが接続するホスト。
		Port: クライアントが接続するポート。
		Path: 接続するエンドポイントのパス。
		UUID および Key: クライアントの識別情報と暗号化キー。
	*/
	cfgBytes, err := genConfig(clientCfg{
		Secure: form.Secure == `true`,
		Host:   form.Host,
		Port:   int(form.Port),
		Path:   form.Path,
		UUID:   hex.EncodeToString(clientUUID),
		Key:    hex.EncodeToString(clientKey),
	})
	//設定が大きすぎる場合（384バイトを超える）、HTTP 413エラーを返す。
	if err != nil {
		if err == ErrTooLargeEntity {
			ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.CONFIG_TOO_LARGE}`})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.CONFIG_GENERATE_FAILED}`})
		return
	}

	//HTTPレスポンスヘッダーの設定
	//クライアントにバイナリファイルをダウンロードさせるため、適切なレスポンスヘッダーを設定。
	// ファイル名はOSに応じて動的に決定。
	ctx.Header(`Accept-Ranges`, `none`)
	ctx.Header(`Content-Transfer-Encoding`, `binary`)
	ctx.Header(`Content-Type`, `application/octet-stream`)
	if stat, err := tpl.Stat(); err == nil {
		ctx.Header(`Content-Length`, strconv.FormatInt(stat.Size(), 10))
	}
	if form.OS == `windows` {
		ctx.Header(`Content-Disposition`, `attachment; filename=client.exe; filename*=UTF-8''client.exe`)
	} else {
		ctx.Header(`Content-Disposition`, `attachment; filename=client; filename*=UTF-8''client`)
	}

	//テンプレート内の設定埋め込み
	//埋め込みの仕組み:
	// テンプレート内に事前定義されたプレースホルダー（384バイトの0x19値）を探す。
	// 見つかった場合、それを生成したクライアント設定（cfgBytes）で置き換える。

	//レスポンスとしてカスタマイズされたバイナリを送信:
	// テンプレートをストリーミングしながら設定を埋め込み、クライアントがダウンロードできるようにする。
	/*
		cfgBuffer は、プレースホルダーとしてテンプレートファイル内に埋め込まれている 384 バイトのデータです。
		ここでは、\x19 という値を 384 バイト分繰り返したバッファが定義されています。
		テンプレートファイル内でこのバッファが存在する部分が、後で生成されるクライアントの設定に置き換えられます。
	*/
	// Find and replace plain buffer with encrypted configuration.
	cfgBuffer := bytes.Repeat([]byte{'\x19'}, 384)

	// ストリーミング送信:
	// テンプレートを1KBごとに読み込んで処理。
	// プレースホルダーを置換しながら、クライアントにリアルタイムでデータを送信。
	prevBuffer := make([]byte, 0)
	for {
		thisBuffer := make([]byte, 1024)
		n, err := tpl.Read(thisBuffer)
		thisBuffer = thisBuffer[:n]
		tempBuffer := append(prevBuffer, thisBuffer...)

		//bytes.Index(tempBuffer, cfgBuffer) を使って、tempBuffer の中に cfgBuffer が含まれているかを探します。
		bufIndex := bytes.Index(tempBuffer, cfgBuffer)
		//含まれていれば、bytes.Replace(tempBuffer, cfgBuffer, cfgBytes, -1) を使って、cfgBuffer を cfgBytes に置き換えます。
		/*
			全体の流れ
			テンプレートファイルを読み込む際に、プレースホルダー（cfgBuffer）を探し、それを生成されたクライアント設定（cfgBytes）に置き換えます。
			プレースホルダーの置換が終わったデータをクライアントに送信し、最終的にカスタマイズされたクライアントバイナリをダウンロードできるようにします。
			この手法により、事前にビルドされたクライアントバイナリにユーザー固有の設定情報を埋め込んで、カスタマイズしたクライアントを配布することが可能です。
		*/
		if bufIndex > -1 {
			tempBuffer = bytes.Replace(tempBuffer, cfgBuffer, cfgBytes, -1)
		}
		ctx.Writer.Write(tempBuffer[:len(prevBuffer)])
		prevBuffer = tempBuffer[len(prevBuffer):]
		if err != nil {
			break
		}
	}
	if len(prevBuffer) > 0 {
		ctx.Writer.Write(prevBuffer)
		prevBuffer = nil
	}

	/*
			動作の流れ
		リクエストを受け取る:
		クライアントがバイナリを生成するための必要なパラメータを送信。
		テンプレートバイナリのロード:
		OSとアーキテクチャに対応するテンプレートをディスクから読み込む。
		設定生成と置換:
		パラメータを元にクライアント設定を生成。
		テンプレート内のプレースホルダーを設定で置き換える。
		クライアントに送信:
		カスタマイズされたバイナリをストリーミング形式でクライアントに送信。
	*/
}

//genConfig 関数: クライアントの設定情報を暗号化し、バッファに埋め込む処理を行います。
/*
役割: クライアント設定を暗号化し、384バイトの固定サイズのデータを生成します。
設定情報を暗号化した後、その長さを2バイトのビッグエンディアン形式でエンコードして先頭に付加します。
最終的に、バッファサイズが不足している場合はランダムなデータで埋めます。
*/
//クライアント設定を暗号化して固定長のバッファ（384バイト）を生成する関数です。生成されたデータは、後でテンプレートバイナリに埋め込まれ、クライアントが使用するための設定データとして利用されます。
func genConfig(cfg clientCfg) ([]byte, error) {
	//設定データをJSON形式に変換
	//cfg（clientCfg構造体）をJSON形式にシリアライズ。
	// シリアライズに失敗した場合、エラーを返して終了。
	data, err := utils.JSON.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	//データの暗号化
	key := utils.GetUUID()
	//暗号化キーとしてランダムなUUID（16バイト）を生成。
	// JSONデータをAESで暗号化。
	data, err = common.EncAES(data, key)
	if err != nil {
		return nil, err
	}

	//暗号化データの構築
	//暗号化キーと暗号化データを結合し、finalバッファを生成。
	// finalの長さが384バイト（予約済みサイズ）を超えた場合、エラーErrTooLargeEntityを返して終了。
	final := append(key, data...)
	if len(final) > 384-2 {
		return nil, ErrTooLargeEntity
	}

	//データ長の追加
	//暗号化されたデータ（final）の長さを計算し、2バイトのビッグエンディアン形式でエンコード。
	// データ長（2バイト）をfinalの先頭に追加。
	// Get the length of encrypted buffer as a 2-byte big-endian integer.
	// And append encrypted buffer to the end of the data length.
	dataLen := big.NewInt(int64(len(final))).Bytes()
	dataLen = append(bytes.Repeat([]byte{'\x00'}, 2-len(dataLen)), dataLen...)

	//バッファの固定長化
	//finalの長さが384バイト未満の場合、ランダムなデータ（UUID）を末尾に追加して埋める。
	// 最終的に384バイトになるよう調整。
	// 暗号化されたバッファの長さが 384 未満の場合、
	// 残りのバイトにランダム バイトを追加します。
	// If the length of encrypted buffer is less than 384,
	// append the remaining bytes with random bytes.
	final = append(dataLen, final...)
	for len(final) < 384 {
		final = append(final, utils.GetUUID()...)
	}

	//384バイトに満たない場合は切り捨てて返す（理論的には384バイトになっている）。
	return final[:384], nil

	/*
			生成されるデータの構造
		先頭2バイト: 暗号化データの長さ（ビッグエンディアン形式）。
		16バイト: 暗号化キー（UUID）。
		暗号化データ: クライアント設定（clientCfg）を暗号化したデータ。
		パディング: 残りをランダムデータで埋め、合計384バイトにする。
		使用用途
		カスタマイズされたクライアント生成:

		生成された384バイトのバッファは、クライアントバイナリの特定の位置に埋め込まれます。
		クライアント起動時に、このバッファを復号化して設定を取得し、動作に必要な接続情報などを利用します。
		セキュリティ:

		AES暗号化により、設定データがバイナリに埋め込まれていても安全に保護されます。
		例: 入力と出力
		入力
		json
		コードをコピーする
		{
		    "Secure": true,
		    "Host": "example.com",
		    "Port": 443,
		    "Path": "/api",
		    "UUID": "1234567890abcdef",
		    "Key": "abcdef1234567890abcdef1234567890"
		}
		出力（構造）
		css
		コードをコピーする
		[2バイト:データ長][16バイト:暗号化キー][暗号化された設定データ][パディング（ランダムデータ）]
		全体は384バイト固定長。

	*/
}
