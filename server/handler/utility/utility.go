package utility

import (
	"Spark/modules"
	"Spark/server/common"
	"Spark/server/config"
	"Spark/utils"
	"Spark/utils/melody"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

/*
WebSocket通信やデバイス管理、およびクライアントとサーバー間のコマンド実行に関する一連のユーティリティ機能を提供しています。
以下に、このコードの主要部分を解説します。


リモートデバイス管理を行うWebサーバーの一部として機能します。リモートデバイスとブラウザとの通信をWebSocketで実現し、クライアントデバイスの管理やコマンド実行、デバイスのヘルスチェックを行うための重要なユーティリティ関数が含まれています。また、暗号化されたデータ通信やバージョン管理などもサポートしており、実用的なデバイス管理ソリューションを提供します。
*/

// 送信関数の型
type Sender func(pack modules.Packet, session *melody.Session) bool

/*
説明: リクエストから接続UUIDまたはデバイスIDを取得して、フォームデータが有効かどうかを確認します。
機能:
form 引数が nil でない場合、リクエストのデータをバインド（デシリアライズ）し、不正なデータがあれば400エラーを返します。
UUIDまたはデバイスIDをチェックし、デバイスが存在しない場合は502エラーを返します。
デバイスが存在する場合は、UUIDを返します。
*/
// CheckForm checks if the form contains the required fields.
// Every request must contain connection UUID or device ID.
func CheckForm(ctx *gin.Context, form any) (string, bool) {
	var base struct {
		Conn   string `json:"uuid" yaml:"uuid" form:"uuid"`
		Device string `json:"device" yaml:"device" form:"device"`
	}
	if form != nil && ctx.ShouldBind(form) != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return ``, false
	}
	if ctx.ShouldBind(&base) != nil || (len(base.Conn) == 0 && len(base.Device) == 0) {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return ``, false
	}
	connUUID, ok := common.CheckDevice(base.Device, base.Conn)
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadGateway, modules.Packet{Code: 1, Msg: `${i18n|COMMON.DEVICE_NOT_EXIST}`})
		return ``, false
	}
	ctx.Request = ctx.Request.WithContext(context.WithValue(ctx.Request.Context(), `ConnUUID`, connUUID))
	return connUUID, true
}

/*
説明: デバイス情報に関するイベント（接続ハンドシェイクやデバイス情報の更新）を処理します。
機能:
クライアントから送信されたデータをデシリアライズして、デバイスの情報を更新します。
新しいデバイスが接続された場合、同じデバイスIDを持つ既存のセッションがあれば、それを終了させます。
デバイスのCPU、RAM、ネットワークなどの情報を更新し、デバイスがオンラインであることをログに記録します。
*/
// OnDevicePack handles events about device info.
// Such as websocket handshake and update device info.
func OnDevicePack(data []byte, session *melody.Session) error {
	var pack struct {
		Code   int            `json:"code,omitempty"`
		Act    string         `json:"act,omitempty"`
		Msg    string         `json:"msg,omitempty"`
		Device modules.Device `json:"data"`
	}
	err := utils.JSON.Unmarshal(data, &pack)
	if err != nil {
		session.Close()
		return err
	}

	addr, ok := session.Get(`Address`)
	if ok {
		pack.Device.WAN = addr.(string)
	} else {
		pack.Device.WAN = `Unknown`
	}

	if pack.Act == `DEVICE_UP` {
		// Check if this device has already connected.
		// If so, then find the session and let client quit.
		// This will keep only one connection remained per device.
		exSession := ``
		common.Devices.IterCb(func(uuid string, device *modules.Device) bool {
			if device.ID == pack.Device.ID {
				exSession = uuid
				target, ok := common.Melody.GetSessionByUUID(uuid)
				if ok {
					common.SendPack(modules.Packet{Act: `OFFLINE`}, target)
					target.Close()
				}
				return false
			}
			return true
		})
		if len(exSession) > 0 {
			common.Devices.Remove(exSession)
		}
		common.Devices.Set(session.UUID, &pack.Device)
		common.Info(nil, `CLIENT_ONLINE`, ``, ``, map[string]any{
			`device`: map[string]any{
				`name`: pack.Device.Hostname,
				`ip`:   pack.Device.WAN,
			},
		})
	} else {
		device, ok := common.Devices.Get(session.UUID)
		if ok {
			device.CPU = pack.Device.CPU
			device.RAM = pack.Device.RAM
			device.Net = pack.Device.Net
			device.Disk = pack.Device.Disk
			device.Uptime = pack.Device.Uptime
		}
	}
	common.SendPack(modules.Packet{Code: 0}, session)
	return nil
}

/*
説明: クライアントが最新バージョンであるかどうかを確認し、必要に応じて更新を提供します。
機能:
クライアントからのOS、アーキテクチャ、コミット情報を取得し、サーバー上の現在のバージョンと比較します。
クライアントが最新でない場合、クライアントに更新データを提供します（client.cfgなどの構成データを含むバイナリファイルの形で）。
*/
// CheckUpdate will check if client need update and return latest client if so.
func CheckUpdate(ctx *gin.Context) {
	var form struct {
		OS     string `form:"os" binding:"required"`
		Arch   string `form:"arch" binding:"required"`
		Commit string `form:"commit" binding:"required"`
	}
	if err := ctx.ShouldBind(&form); err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	if form.Commit == config.COMMIT {
		ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
		common.Warn(ctx, `CLIENT_UPDATE`, `success`, `latest`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}
	tpl, err := os.Open(fmt.Sprintf(config.BuiltPath, form.OS, form.Arch))
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, modules.Packet{Code: 1, Msg: `${i18n|GENERATOR.NO_PREBUILT_FOUND}`})
		common.Warn(ctx, `CLIENT_UPDATE`, `fail`, `no prebuild asset`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}
	defer tpl.Close()

	const MaxBodySize = 384 // This is size of client config buffer.
	if ctx.Request.ContentLength > MaxBodySize {
		ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, modules.Packet{Code: 1})
		common.Warn(ctx, `CLIENT_UPDATE`, `fail`, `config too large`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}
	body, err := ctx.GetRawData()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: 1})
		common.Warn(ctx, `CLIENT_UPDATE`, `fail`, `read config fail`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}
	session := common.CheckClientReq(ctx)
	if session == nil {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, modules.Packet{Code: 1})
		common.Warn(ctx, `CLIENT_UPDATE`, `fail`, `check config fail`, map[string]any{
			`client`: map[string]any{
				`os`:     form.OS,
				`arch`:   form.Arch,
				`commit`: form.Commit,
			},
			`server`: config.COMMIT,
		})
		return
	}

	common.Info(ctx, `CLIENT_UPDATE`, `success`, `updating`, map[string]any{
		`client`: map[string]any{
			`os`:     form.OS,
			`arch`:   form.Arch,
			`commit`: form.Commit,
		},
		`server`: config.COMMIT,
	})

	ctx.Header(`Spark-Commit`, config.COMMIT)
	ctx.Header(`Accept-Ranges`, `none`)
	ctx.Header(`Content-Transfer-Encoding`, `binary`)
	ctx.Header(`Content-Type`, `application/octet-stream`)
	if stat, err := tpl.Stat(); err == nil {
		ctx.Header(`Content-Length`, strconv.FormatInt(stat.Size(), 10))
	}
	cfgBuffer := bytes.Repeat([]byte{'\x19'}, 384)
	prevBuffer := make([]byte, 0)
	for {
		thisBuffer := make([]byte, 1024)
		n, err := tpl.Read(thisBuffer)
		thisBuffer = thisBuffer[:n]
		tempBuffer := append(prevBuffer, thisBuffer...)
		bufIndex := bytes.Index(tempBuffer, cfgBuffer)
		if bufIndex > -1 {
			tempBuffer = bytes.Replace(tempBuffer, cfgBuffer, body, -1)
		}
		ctx.Writer.Write(tempBuffer[:len(prevBuffer)])
		prevBuffer = tempBuffer[len(prevBuffer):]
		if err != nil {
			break
		}
	}
	if len(prevBuffer) > 0 {
		ctx.Writer.Write(prevBuffer)
		prevBuffer = []byte{}
	}
}

/*
説明: 指定されたコマンドをリモートデバイス上で実行します。
機能:
コマンドと引数をリクエストから取得し、ターゲットデバイスに対してコマンドを送信します。
5秒以内にレスポンスが返ってこない場合、タイムアウトエラーを返します。
*/
// ExecDeviceCmd execute command on device.
func ExecDeviceCmd(ctx *gin.Context) {
	var form struct {
		Cmd  string `json:"cmd" yaml:"cmd" form:"cmd" binding:"required"`
		Args string `json:"args" yaml:"args" form:"args"`
	}
	target, ok := CheckForm(ctx, &form)
	if !ok {
		return
	}
	if len(form.Cmd) == 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	trigger := utils.GetStrUUID()
	common.SendPackByUUID(modules.Packet{Act: `COMMAND_EXEC`, Data: gin.H{`cmd`: form.Cmd, `args`: form.Args}, Event: trigger}, target)
	ok = common.AddEventOnce(func(p modules.Packet, _ *melody.Session) {
		if p.Code != 0 {
			common.Warn(ctx, `EXEC_COMMAND`, `fail`, p.Msg, map[string]any{
				`cmd`:  form.Cmd,
				`args`: form.Args,
			})
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		} else {
			common.Info(ctx, `EXEC_COMMAND`, `success`, ``, map[string]any{
				`cmd`:  form.Cmd,
				`args`: form.Args,
			})
			ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
		}
	}, target, trigger, 5*time.Second)
	if !ok {
		common.Warn(ctx, `EXEC_COMMAND`, `fail`, `timeout`, map[string]any{
			`cmd`:  form.Cmd,
			`args`: form.Args,
		})
		ctx.AbortWithStatusJSON(http.StatusGatewayTimeout, modules.Packet{Code: 1, Msg: `${i18n|COMMON.RESPONSE_TIMEOUT}`})
	}
}

/*
説明: 接続されているすべてのクライアントデバイスの情報を取得して返します。
機能:
common.Devices に保存されているすべてのデバイス情報を取得し、HTTPレスポンスとして返します。
*/
// GetDevices will return all info about all clients.
func GetDevices(ctx *gin.Context) {
	devices := map[string]any{}
	common.Devices.IterCb(func(uuid string, device *modules.Device) bool {
		devices[uuid] = *device
		return true
	})
	ctx.JSON(http.StatusOK, modules.Packet{Code: 0, Data: devices})
}

/*
説明: 特定のコマンド（ロック、ログオフ、シャットダウンなど）をクライアントデバイスに送信します。
機能:
act パラメータを取得し、それに基づいてリモートデバイスに対してコマンドを実行します。
クライアントがオフラインの場合でも、コマンドが成功したと見なします。
*/
// CallDevice will call client with command from browser.
func CallDevice(ctx *gin.Context) {
	act := strings.ToUpper(ctx.Param(`act`))
	if len(act) == 0 {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
		return
	}
	{
		actions := []string{`LOCK`, `LOGOFF`, `HIBERNATE`, `SUSPEND`, `RESTART`, `SHUTDOWN`, `OFFLINE`}
		ok := false
		for _, v := range actions {
			if v == act {
				ok = true
				break
			}
		}
		if !ok {
			common.Warn(ctx, `CALL_DEVICE`, `fail`, `invalid act`, map[string]any{
				`act`: act,
			})
			ctx.AbortWithStatusJSON(http.StatusBadRequest, modules.Packet{Code: -1, Msg: `${i18n|COMMON.INVALID_PARAMETER}`})
			return
		}
	}
	connUUID, ok := CheckForm(ctx, nil)
	if !ok {
		return
	}
	trigger := utils.GetStrUUID()
	common.SendPackByUUID(modules.Packet{Act: act, Event: trigger}, connUUID)
	ok = common.AddEventOnce(func(p modules.Packet, _ *melody.Session) {
		if p.Code != 0 {
			common.Warn(ctx, `CALL_DEVICE`, `fail`, p.Msg, map[string]any{
				`act`: act,
			})
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, modules.Packet{Code: 1, Msg: p.Msg})
		} else {
			common.Info(ctx, `CALL_DEVICE`, `success`, ``, map[string]any{
				`act`: act,
			})
			ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
		}
	}, connUUID, trigger, 5*time.Second)
	if !ok {
		//This means the client is offline.
		//So we take this as a success.
		common.Info(ctx, `CALL_DEVICE`, `success`, ``, map[string]any{
			`act`: act,
		})
		ctx.JSON(http.StatusOK, modules.Packet{Code: 0})
	}
}

/*
説明: データをセッションごとに一意な「Secret」を使用してシンプルなXOR暗号化を行います。
機能:
セッションの Secret を使用して、データをXOR方式で暗号化または復号化します。
*/
func SimpleEncrypt(data []byte, session *melody.Session) []byte {
	temp, ok := session.Get(`Secret`)
	if !ok {
		return nil
	}
	secret := temp.([]byte)
	return utils.XOR(data, secret)
}

func SimpleDecrypt(data []byte, session *melody.Session) []byte {
	temp, ok := session.Get(`Secret`)
	if !ok {
		return nil
	}
	secret := temp.([]byte)
	return utils.XOR(data, secret)
}

/*
説明: WebSocket接続のヘルスチェックを行います。
機能:
一定期間応答がないWebSocketセッションをクローズします。
各セッションに対して PING パケットを送信し、応答がないセッションをクローズします。
最後のメッセージが受信されてから300秒以上経過したセッションも終了させます。
*/
func WSHealthCheck(container *melody.Melody, sender Sender) {
	const MaxIdleSeconds = 300
	ping := func(uuid string, s *melody.Session) {
		if !sender(modules.Packet{Act: `PING`}, s) {
			s.Close()
		}
	}
	for now := range time.NewTicker(60 * time.Second).C {
		timestamp := now.Unix()
		// stores sessions to be disconnected
		queue := make([]*melody.Session, 0)
		container.IterSessions(func(uuid string, s *melody.Session) bool {
			go ping(uuid, s)
			val, ok := s.Get(`LastPack`)
			if !ok {
				queue = append(queue, s)
				return true
			}
			lastPack, ok := val.(int64)
			if !ok {
				queue = append(queue, s)
				return true
			}
			if timestamp-lastPack > MaxIdleSeconds {
				queue = append(queue, s)
			}
			return true
		})
		for i := 0; i < len(queue); i++ {
			queue[i].Close()
		}
	}
}
