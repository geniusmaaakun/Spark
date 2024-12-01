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

// メッセージのサイズ上限
// 66560
const MaxMessageSize = (2 << 15) + 1024

/*
Melody: WebSocketセッションを管理するmelodyライブラリのインスタンス。この変数を通じて、セッションの管理やメッセージの送受信を行います。
Devices: cmapライブラリ（スレッドセーフなマップ）を使用して、デバイス情報を管理するためのデータ構造です。デバイスごとにセッションやデータが管理されます。
*/
var Melody = melody.New() //wsのセッション管理の構造体
var Devices = cmap.New[*modules.Device]()

// SendPackByUUID: 指定されたUUIDを持つWebSocketセッションに対して、パケットを送信します。
func SendPackByUUID(pack modules.Packet, uuid string) bool {
	// melodyからsessionの取得
	session, ok := Melody.GetSessionByUUID(uuid)
	if !ok {
		return false
	}
	// packetの送信
	return SendPack(pack, session)
}

// SendPack: WebSocketセッションにパケットを送信する際に、まずパケットをJSONに変換し、暗号化（Encrypt）した後、バイナリデータとして送信します。
func SendPack(pack modules.Packet, session *melody.Session) bool {
	if session == nil {
		return false
	}
	// json化
	data, err := utils.JSON.Marshal(pack)
	if err != nil {
		return false
	}
	// 暗号化
	data, ok := Encrypt(data, session)
	if !ok {
		return false
	}
	// パケットの送信
	err = session.WriteBinary(data)
	return err == nil
}

// Encrypt: セッションごとに保存されているSecretキー（暗号鍵）を使用して、データを暗号化します。暗号化にはutils.Encrypt（おそらくAES暗号化）を使用しています。
func Encrypt(data []byte, session *melody.Session) ([]byte, bool) {
	//sessionからデータを取得
	temp, ok := session.Get(`Secret`)
	if !ok {
		return nil, false
	}
	//byteに型アサーション
	secret := temp.([]byte)
	// 暗号化
	dec, err := utils.Encrypt(data, secret)
	if err != nil {
		return nil, false
	}
	return dec, true
}

// Decrypt: 逆に、受信したデータをセッションのSecretキーを使用して復号化します。
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

// GetAddrIP: net.Addr型のアドレスから、TCPやUDP、IPアドレスを取得します。
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

/*
GetRealIP:
ミドルウェアや事前処理で ClientIP を設定している場合に効果的。
高速だが、依存する環境が限定的。

GetRemoteAddr:
プロキシ環境や多様なネットワーク構成でも動作する汎用的なIP取得ロジック。
若干の処理オーバーヘッドがある。
*/
// GetRealIP: Ginフレームワークを使用して、クライアントの本当のIPアドレスを取得します。X-Forwarded-ForやX-Real-IPヘッダーも考慮して、プロキシ環境でのクライアントIPを正確に取得します。
func GetRealIP(ctx *gin.Context) string {
	addr, ok := ctx.Request.Context().Value(`ClientIP`).(string)
	if !ok {
		return GetRemoteAddr(ctx)
	}
	return addr
}

// GetRemoteAddr は、HTTPリクエストを送信してきたクライアントのIPアドレスを取得するための処理を行います。
// 以下でコードを詳細に解説します。
func GetRemoteAddr(ctx *gin.Context) string {
	//クライアント（リクエスト送信者）のIPアドレスを取得する必要がある場合に使用。
	if remote, ok := ctx.RemoteIP(); ok {
		//リモートアドレスがループバックアドレス（例: 127.0.0.1）であるかどうかを判定します。
		//ループバックの場合、実際のクライアントIPはリクエストヘッダーの X-Forwarded-For または X-Real-IP に含まれている可能性があるため、それをチェックします。
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
			//IPv4の場合は To4() を使用し、IPを文字列形式に変換して返します。
			if ip := remote.To4(); ip != nil {
				return ip.String()
			}
			//IPv6の場合は To16() を使用して文字列形式に変換します。
			if ip := remote.To16(); ip != nil {
				return ip.String()
			}
		}
	}

	//プロキシサーバーやロードバランサーを介している場合でも、クライアントのIPアドレスを正しく取得しようとする。
	//ctx.RemoteIP() が成功しなかった場合に備え、ctx.Request.RemoteAddr を使ってIPアドレスを手動で解析します。
	remote := net.ParseIP(ctx.Request.RemoteAddr)
	if remote != nil {
		// リモートアドレスがローカル（ループバックアドレス）の場合
		if remote.IsLoopback() {
			//X-Forwarded-For:
			//プロキシサーバーが実際のクライアントIPをこのヘッダーに含める。
			//複数のプロキシを経由する場合、カンマ区切りで複数のIPが記載される。
			forwarded := ctx.GetHeader(`X-Forwarded-For`)
			if len(forwarded) > 0 {
				return forwarded
			}
			//X-Real-IP:
			//特定のプロキシがクライアントのIPアドレスを簡潔に設定するために使用。
			realIP := ctx.GetHeader(`X-Real-IP`)
			if len(realIP) > 0 {
				return realIP
			}
		} else {
			// IPv4の場合
			if ip := remote.To4(); ip != nil {
				return ip.String()
			}
			// IPv6の場合
			if ip := remote.To16(); ip != nil {
				return ip.String()
			}
		}
	}
	//クライアントのリモートIPアドレスを string 型で返す。
	//ctx.Request.RemoteAddr が host:port の形式で提供されている場合、ポート部分を除去してIPアドレスだけを返します。
	addr := ctx.Request.RemoteAddr
	//strings.LastIndex(addr, ":"):
	//最後の : を見つけて、ポート部分を特定します。
	//addr[:pos] でポートを除外し、[]（IPv6アドレス用の括弧）も取り除きます。
	if pos := strings.LastIndex(addr, `:`); pos > -1 {
		return strings.Trim(addr[:pos], `[]`)
	}
	return addr
}

// CheckClientReq: GinのコンテキストからSecretヘッダーを取り出し、これがWebSocketセッションのSecretと一致するかを確認します。クライアントが正しい認証情報を持っているかどうかを検証するための機能です。
func CheckClientReq(ctx *gin.Context) *melody.Session {
	//HTTPリクエストヘッダーからSecretを取得します。
	//取得したSecretを16進文字列からバイト配列に変換します。
	//Secretの長さが32バイト（256ビット）でない場合は不正とみなし、nilを返して終了します。
	secret, err := hex.DecodeString(ctx.GetHeader(`Secret`))
	if err != nil || len(secret) != 32 {
		return nil
	}
	//サーバー側に保持されているすべてのWebSocketセッションをMelody.IterSessionsでイテレートします。
	var result *melody.Session = nil
	Melody.IterSessions(func(uuid string, s *melody.Session) bool {
		//Secretが設定されているか
		if val, ok := s.Get(`Secret`); ok {
			//クライアントのSecretとの一致を確認
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

// CheckDevice: 指定されたデバイスIDと接続UUIDを使って、デバイスが既に登録されているかどうかを確認します。登録されていない場合、新しいUUIDを返します。
// デバイスのID（deviceID）と接続UUID（connUUID）を元に、サーバーがデバイスを認識しているかどうかを確認します。また、必要に応じて、該当する接続UUIDを取得します。
// deviceID string:
// デバイスの一意の識別子。
// connUUID string:
// 接続に関連付けられたUUID。
// 空の場合はdeviceIDを使用してデバイスを検索します。
func CheckDevice(deviceID, connUUID string) (string, bool) {
	//接続UUIDが指定されている場合
	if len(connUUID) > 0 {
		//Devices.Hasで接続UUIDの存在を確認:
		if !Devices.Has(connUUID) {
			return connUUID, true
		}
	} else {
		//接続UUIDが指定されていない場合
		//一時的なUUID変数tempConnUUIDを初期化
		tempConnUUID := ``
		//Devices.IterCbでデバイスを検索
		//Devices.IterCbは、登録されたすべてのデバイスをコールバック関数でループ処理します。
		// ループ内でdevice.IDが指定されたdeviceIDと一致するか確認します。
		// 一致する場合:
		// 該当デバイスのUUIDをtempConnUUIDに保存し、return falseでループを終了します。
		// 一致しない場合:
		// 次のデバイスに進むためreturn trueを返します。
		Devices.IterCb(func(uuid string, device *modules.Device) bool {
			if device.ID == deviceID {
				tempConnUUID = uuid
				return false
			}
			return true
		})
		//検索結果を返す:
		// tempConnUUIDにUUIDが設定されている場合はそれを返し、len(tempConnUUID) > 0（true）を返します。
		// 該当デバイスが見つからなかった場合、空文字列とfalseを返します。
		return tempConnUUID, len(tempConnUUID) > 0
	}
	return ``, false
}

// EncAES: AES暗号化を行います。データとキーを使ってAES-CTRモードでデータを暗号化し、MD5ハッシュを生成してから暗号化されたデータとハッシュを結合して返します。
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

// DecAES: 逆に、AES-CTRモードでデータを復号化し、MD5ハッシュによる検証を行います。
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
