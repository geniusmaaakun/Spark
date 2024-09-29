package common

import (
	"Spark/modules"
	"Spark/utils"
	"Spark/utils/cmap"
	"Spark/utils/melody"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

/*
Webアプリケーションにおいてセッション管理、パケットの暗号化/復号化、デバイス管理、クライアントIP取得などのユーティリティ機能を提供するGo言語のパッケージです。
コード全体は、melodyライブラリを使用してWebSocketセッションを管理しつつ、AES暗号化やデバイス認証などの機能をサポートしています。


WebSocketセッションを使った通信において、データの暗号化、パケットの送信、クライアント認証、デバイス管理、IPアドレスの取得など、複数のユーティリティ機能を提供しています。melodyを使ってWebSocketのセッションを管理し、暗号化やセッション管理を通じて、セキュリティを強化した通信を実現しています。
*/

//メッセージのサイズ上限
const MaxMessageSize = (2 << 15) + 1024

/*
Melody: WebSocketセッションを管理するmelodyライブラリのインスタンス。この変数を通じて、セッションの管理やメッセージの送受信を行います。
Devices: cmapライブラリ（スレッドセーフなマップ）を使用して、デバイス情報を管理するためのデータ構造です。デバイスごとにセッションやデータが管理されます。
*/
var Melody = melody.New()
var Devices = cmap.New[*modules.Device]()

//SendPackByUUID: 指定されたUUIDを持つWebSocketセッションに対して、パケットを送信します。
func SendPackByUUID(pack modules.Packet, uuid string) bool {
	session, ok := Melody.GetSessionByUUID(uuid)
	if !ok {
		return false
	}
	return SendPack(pack, session)
}

//SendPack: WebSocketセッションにパケットを送信する際に、まずパケットをJSONに変換し、暗号化（Encrypt）した後、バイナリデータとして送信します。
func SendPack(pack modules.Packet, session *melody.Session) bool {
	if session == nil {
		return false
	}
	data, err := utils.JSON.Marshal(pack)
	if err != nil {
		return false
	}
	data, ok := Encrypt(data, session)
	if !ok {
		return false
	}
	err = session.WriteBinary(data)
	return err == nil
}

//Encrypt: セッションごとに保存されているSecretキー（暗号鍵）を使用して、データを暗号化します。暗号化にはutils.Encrypt（おそらくAES暗号化）を使用しています。
func Encrypt(data []byte, session *melody.Session) ([]byte, bool) {
	temp, ok := session.Get(`Secret`)
	if !ok {
		return nil, false
	}
	secret := temp.([]byte)
	dec, err := utils.Encrypt(data, secret)
	if err != nil {
		return nil, false
	}
	return dec, true
}

//Decrypt: 逆に、受信したデータをセッションのSecretキーを使用して復号化します。
func Decrypt(data []byte, session *melody.Session) ([]byte, bool) {
	temp, ok := session.Get(`Secret`)
	if !ok {
		return nil, false
	}
	secret := temp.([]byte)
	dec, err := utils.Decrypt(data, secret)
	if err != nil {
		return nil, false
	}
	return dec, true
}

//GetAddrIP: net.Addr型のアドレスから、TCPやUDP、IPアドレスを取得します。
func GetAddrIP(addr net.Addr) string {
	switch addr.(type) {
	case *net.TCPAddr:
		return addr.(*net.TCPAddr).IP.String()
	case *net.UDPAddr:
		return addr.(*net.UDPAddr).IP.String()
	case *net.IPAddr:
		return addr.(*net.IPAddr).IP.String()
	default:
		return addr.String()
	}
}

//GetRealIP: Ginフレームワークを使用して、クライアントの本当のIPアドレスを取得します。X-Forwarded-ForやX-Real-IPヘッダーも考慮して、プロキシ環境でのクライアントIPを正確に取得します。
func GetRealIP(ctx *gin.Context) string {
	addr, ok := ctx.Request.Context().Value(`ClientIP`).(string)
	if !ok {
		return GetRemoteAddr(ctx)
	}
	return addr
}

func GetRemoteAddr(ctx *gin.Context) string {
	if remote, ok := ctx.RemoteIP(); ok {
		if remote.IsLoopback() {
			forwarded := ctx.GetHeader(`X-Forwarded-For`)
			if len(forwarded) > 0 {
				return forwarded
			}
			realIP := ctx.GetHeader(`X-Real-IP`)
			if len(realIP) > 0 {
				return realIP
			}
		} else {
			if ip := remote.To4(); ip != nil {
				return ip.String()
			}
			if ip := remote.To16(); ip != nil {
				return ip.String()
			}
		}
	}

	remote := net.ParseIP(ctx.Request.RemoteAddr)
	if remote != nil {
		if remote.IsLoopback() {
			forwarded := ctx.GetHeader(`X-Forwarded-For`)
			if len(forwarded) > 0 {
				return forwarded
			}
			realIP := ctx.GetHeader(`X-Real-IP`)
			if len(realIP) > 0 {
				return realIP
			}
		} else {
			if ip := remote.To4(); ip != nil {
				return ip.String()
			}
			if ip := remote.To16(); ip != nil {
				return ip.String()
			}
		}
	}
	addr := ctx.Request.RemoteAddr
	if pos := strings.LastIndex(addr, `:`); pos > -1 {
		return strings.Trim(addr[:pos], `[]`)
	}
	return addr
}

//CheckClientReq: GinのコンテキストからSecretヘッダーを取り出し、これがWebSocketセッションのSecretと一致するかを確認します。クライアントが正しい認証情報を持っているかどうかを検証するための機能です。
func CheckClientReq(ctx *gin.Context) *melody.Session {
	secret, err := hex.DecodeString(ctx.GetHeader(`Secret`))
	if err != nil || len(secret) != 32 {
		return nil
	}
	var result *melody.Session = nil
	Melody.IterSessions(func(uuid string, s *melody.Session) bool {
		if val, ok := s.Get(`Secret`); ok {
			// Check if there's a connection matches this secret.
			if b, ok := val.([]byte); ok && bytes.Equal(b, secret) {
				result = s
				return false
			}
		}
		return true
	})
	return result
}

//CheckDevice: 指定されたデバイスIDと接続UUIDを使って、デバイスが既に登録されているかどうかを確認します。登録されていない場合、新しいUUIDを返します。
func CheckDevice(deviceID, connUUID string) (string, bool) {
	if len(connUUID) > 0 {
		if !Devices.Has(connUUID) {
			return connUUID, true
		}
	} else {
		tempConnUUID := ``
		Devices.IterCb(func(uuid string, device *modules.Device) bool {
			if device.ID == deviceID {
				tempConnUUID = uuid
				return false
			}
			return true
		})
		return tempConnUUID, len(tempConnUUID) > 0
	}
	return ``, false
}

//EncAES: AES暗号化を行います。データとキーを使ってAES-CTRモードでデータを暗号化し、MD5ハッシュを生成してから暗号化されたデータとハッシュを結合して返します。
func EncAES(data []byte, key []byte) ([]byte, error) {
	hash, _ := utils.GetMD5(data)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(block, hash)
	encBuffer := make([]byte, len(data))
	stream.XORKeyStream(encBuffer, data)
	return append(hash, encBuffer...), nil
}

//DecAES: 逆に、AES-CTRモードでデータを復号化し、MD5ハッシュによる検証を行います。
func DecAES(data []byte, key []byte) ([]byte, error) {
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
