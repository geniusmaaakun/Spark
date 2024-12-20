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
func CheckClient(ctx *gin.Context) {
	var form struct {
		OS     string `json:"os" yaml:"os" form:"os" binding:"required"`
		Arch   string `json:"arch" yaml:"arch" form:"arch" binding:"required"`
		Host   string `json:"host" yaml:"host" form:"host" binding:"required"`
		Port   uint16 `json:"port" yaml:"port" form:"port" binding:"required"`
		Path   string `json:"path" yaml:"path" form:"path" binding:"required"`
		Secure string `json:"secure" yaml:"secure" form:"secure"`
	}
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	_, err := os.Stat(fmt.Sprintf(config.BuiltPath, form.OS, form.Arch))
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.NO_PREBUILT_FOUND}`})
		return
	}
	_, err = genConfig(clientCfg{
		Secure: form.Secure == `true`,
		Host:   form.Host,
		Port:   int(form.Port),
		Path:   form.Path,
		UUID:   strings.Repeat(`FF`, 16),
		Key:    strings.Repeat(`FF`, 32),
	})
	if err != nil {
		if err == ErrTooLargeEntity {
			ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.CONFIG_TOO_LARGE}`})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.CONFIG_GENERATE_FAILED}`})
		return
	}
	ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
}

//GenerateClient 関数: クライアントのバイナリファイルを生成し、設定情報を埋め込んだバイナリファイルをレスポンスとして返します。
/*
役割: クライアントの実行ファイル（OSとアーキテクチャに対応したバイナリファイル）を生成し、設定情報を埋め込んでレスポンスとして送信します。
ファイルをバイト単位で読み込み、指定された位置にクライアントの設定情報を暗号化して埋め込みます。
ファイルはストリーミングで送信され、HTTPヘッダーには適切なファイル名やコンテンツ情報が設定されます。
*/
func GenerateClient(ctx *gin.Context) {
	var form struct {
		OS     string `json:"os" yaml:"os" form:"os" binding:"required"`
		Arch   string `json:"arch" yaml:"arch" form:"arch" binding:"required"`
		Host   string `json:"host" yaml:"host" form:"host" binding:"required"`
		Port   uint16 `json:"port" yaml:"port" form:"port" binding:"required"`
		Path   string `json:"path" yaml:"path" form:"path" binding:"required"`
		Secure string `json:"secure" yaml:"secure" form:"secure"`
	}
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	// templateのバイナリファイルを読み込む
	tpl, err := os.Open(fmt.Sprintf(config.BuiltPath, form.OS, form.Arch))
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.NO_PREBUILT_FOUND}`})
		return
	}
	defer tpl.Close()
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
	if err != nil {
		if err == ErrTooLargeEntity {
			ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.CONFIG_TOO_LARGE}`})
			return
		}
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.CONFIG_GENERATE_FAILED}`})
		return
	}
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

	/*
		cfgBuffer は、プレースホルダーとしてテンプレートファイル内に埋め込まれている 384 バイトのデータです。
		ここでは、\x19 という値を 384 バイト分繰り返したバッファが定義されています。
		テンプレートファイル内でこのバッファが存在する部分が、後で生成されるクライアントの設定に置き換えられます。
	*/
	// Find and replace plain buffer with encrypted configuration.
	cfgBuffer := bytes.Repeat([]byte{'\x19'}, 384)
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
}

//genConfig 関数: クライアントの設定情報を暗号化し、バッファに埋め込む処理を行います。
/*
役割: クライアント設定を暗号化し、384バイトの固定サイズのデータを生成します。
設定情報を暗号化した後、その長さを2バイトのビッグエンディアン形式でエンコードして先頭に付加します。
最終的に、バッファサイズが不足している場合はランダムなデータで埋めます。
*/
func genConfig(cfg clientCfg) ([]byte, error) {
	data, err := utils.JSON.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	key := utils.GetUUID()
	data, err = common.EncAES(data, key)
	if err != nil {
		return nil, err
	}
	final := append(key, data...)
	if len(final) > 384-2 {
		return nil, ErrTooLargeEntity
	}

	// Get the length of encrypted buffer as a 2-byte big-endian integer.
	// And append encrypted buffer to the end of the data length.
	dataLen := big.NewInt(int64(len(final))).Bytes()
	dataLen = append(bytes.Repeat([]byte{'\x00'}, 2-len(dataLen)), dataLen...)

	// 暗号化されたバッファの長さが 384 未満の場合、
	// 残りのバイトにランダム バイトを追加します。
	// If the length of encrypted buffer is less than 384,
	// append the remaining bytes with random bytes.
	final = append(dataLen, final...)
	for len(final) < 384 {
		final = append(final, utils.GetUUID()...)
	}
	return final[:384], nil
}
