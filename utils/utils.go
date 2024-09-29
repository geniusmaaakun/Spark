package utils

/*
crypto/aes, crypto/cipher: AES暗号化を行うためのライブラリ。
crypto/md5: MD5ハッシュを計算するためのライブラリ。
encoding/hex: バイト配列を16進数表記に変換するためのライブラリ。
errors: エラーハンドリング用の標準ライブラリ。
fmt: フォーマットされた入出力を提供するライブラリ。
jsoniter: 高速なJSON操作を提供する外部ライブラリ。
reflect, unsafe: 低レベルメモリ操作のためのライブラリ。
*/
import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
)

/*
JSON: JSON操作用の設定。HTMLエスケープを行わず、マップのキーをソートする設定。
ErrEntityInvalid: エンティティが無効であることを示すエラー。
ErrFailedVerification: 検証に失敗したことを示すエラー。
*/
var (
	JSON = jsoniter.Config{EscapeHTML: false, SortMapKeys: true, ValidateJsonRawMessage: true}.Froze()

	ErrEntityInvalid      = errors.New(`common.ENTITY_INVALID`)
	ErrFailedVerification = errors.New(`common.ENTITY_CHECK_FAILED`)
)

//If: 条件付きの値選択を行う関数。条件 b が真なら t、偽なら f を返す。
func If[T any](b bool, t, f T) T {
	if b {
		return t
	}
	return f
}

//Min, Max: 与えられた2つの値のうち、最小値および最大値を返すジェネリック関数。
func Min[T int | int32 | int64 | uint | uint32 | uint64 | float32 | float64](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func Max[T int | int32 | int64 | uint | uint32 | uint64 | float32 | float64](a, b T) T {
	if a > b {
		return a
	}
	return b
}

//XOR: XOR暗号を用いてデータを暗号化する関数。dataとkeyの各バイトをXOR演算で暗号化します。
func XOR(data []byte, key []byte) []byte {
	// keyが空の場合はdataをそのまま返す
	if len(key) == 0 {
		return data
	}
	// dataの各バイトに対してkeyの対応するバイトでXOR演算を行う
	for i := 0; i < len(data); i++ {
		data[i] = data[i] ^ key[i%len(key)]
	}
	return data
}

//GenRandByte: nバイトのランダムデータを生成する関数。暗号用の強力な乱数生成を使用。
func GenRandByte(n int) []byte {
	// nバイトのバッファを作成し、乱数で埋める
	secBuffer := make([]byte, n)
	// 暗号用の強力な乱数生成を使用
	rand.Reader.Read(secBuffer)
	return secBuffer
}

//GetStrUUID: 16バイトのランダムデータを16進数の文字列形式で返すUUID生成関数。
func GetStrUUID() string {
	// 16バイトのランダムデータを生成し、16進数の文字列形式で返す
	return hex.EncodeToString(GenRandByte(16))
}

//GetUUID: 16バイトのランダムデータをそのまま返すUUID生成関数。
func GetUUID() []byte {
	return GenRandByte(16)
}

//GetMD5: 入力データのMD5ハッシュ値とその16進数文字列を返す関数。
func GetMD5(data []byte) ([]byte, string) {
	hash := md5.New()
	hash.Write(data)
	result := hash.Sum(nil)
	hash.Reset()
	return result, hex.EncodeToString(result)
}

//?
// AES 共通鍵暗号化
//Encrypt: AES-CTRモードでデータを暗号化する関数。MD5を用いてデータのハッシュを計算し、暗号化に使用。
func Encrypt(data []byte, key []byte) ([]byte, error) {
	//fmt.Println(`Send: `, string(data))

	// nonceを生成し、データに追加
	nonce := make([]byte, 64)
	// 暗号用の強力な乱数生成を使用
	rand.Reader.Read(nonce)
	data = append(data, nonce...)

	// データのMD5ハッシュを計算
	hash, _ := GetMD5(data)
	// aes.NewCipherで暗号化ブロックを生成
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// 暗号化ブロックとhashを用いてCTRモードのストリームを生成
	stream := cipher.NewCTR(block, hash)
	// データを暗号化
	encBuffer := make([]byte, len(data))
	stream.XORKeyStream(encBuffer, data)
	// hashと暗号化データを返す
	return append(hash, encBuffer...), nil
}

//Decrypt: 暗号化されたデータを復号し、ハッシュを検証してデータの整合性を確認します。
func Decrypt(data []byte, key []byte) ([]byte, error) {
	// MD5[16 bytes] + Data[n bytes] + Nonce[64 bytes]

	// データの長さが16+64未満の場合はエラーを返す
	dataLen := len(data)
	if dataLen <= 16+64 {
		return nil, ErrEntityInvalid
	}
	// aes.NewCipherで暗号化ブロックを生成
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// データの16バイト以降を復号
	// data[:16]はハッシュ値
	// data[16:]は暗号化されたデータ
	stream := cipher.NewCTR(block, data[:16])
	// decBufferはデータの16バイト以降を復号した結果
	decBuffer := make([]byte, dataLen-16)
	stream.XORKeyStream(decBuffer, data[16:])

	// データのハッシュを計算し、検証
	hash, _ := GetMD5(decBuffer)
	if !bytes.Equal(hash, data[:16]) {
		data = nil
		decBuffer = nil
		return nil, ErrFailedVerification
	}
	// データのハッシュを削除
	data = nil
	// 16バイトのハッシュと64バイトのNonceを削除
	decBuffer = decBuffer[:dataLen-16-64]

	//fmt.Println(`Recv: `, string(decBuffer[:dataLen-16-64]))
	return decBuffer, nil
}

//FormatSize: バイトサイズを人間が読みやすい形式（B, KB, MB, etc.）でフォーマットする関数。
func FormatSize(size int64) string {
	sizes := []string{`B`, `KB`, `MB`, `GB`, `TB`, `PB`, `EB`, `ZB`, `YB`}
	i := 0
	for size >= 1024 && i < len(sizes)-1 {
		size /= 1024
		i++
	}
	return fmt.Sprintf(`%d%s`, size, sizes[i])
}

//?
//**BytesToStringとStringToBytes**は、メモリコピーを避けて高速にバイト列と文字列を相互変換する関数です。
func BytesToString(b []byte, r ...int) string {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bytesPtr := sh.Data
	bytesLen := sh.Len
	switch len(r) {
	case 1:
		r[0] = If(r[0] > bytesLen, bytesLen, r[0])
		bytesLen -= r[0]
		bytesPtr += uintptr(r[0])
	case 2:
		r[0] = If(r[0] > bytesLen, bytesLen, r[0])
		bytesLen = If(r[1] > bytesLen, bytesLen, r[1]) - r[0]
		bytesPtr += uintptr(r[0])
	}
	return *(*string)(unsafe.Pointer(&reflect.StringHeader{
		Data: bytesPtr,
		Len:  bytesLen,
	}))
}

func StringToBytes(s string, r ...int) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	strPtr := sh.Data
	strLen := sh.Len
	switch len(r) {
	case 1:
		r[0] = If(r[0] > strLen, strLen, r[0])
		strLen -= r[0]
		strPtr += uintptr(r[0])
	case 2:
		r[0] = If(r[0] > strLen, strLen, r[0])
		strLen = If(r[1] > strLen, strLen, r[1]) - r[0]
		strPtr += uintptr(r[0])
	}
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: strPtr,
		Len:  strLen,
		Cap:  strLen,
	}))
}

//スライスの先頭、末尾、部分を取得するための関数群。
func GetSlicePrefix[T any](data *[]T, n int) *[]T {
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(data))
	return (*[]T)(unsafe.Pointer(&reflect.SliceHeader{
		Data: sliceHeader.Data,
		Len:  n,
		Cap:  n,
	}))
}

func GetSliceSuffix[T any](data *[]T, n int) *[]T {
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(data))
	return (*[]T)(unsafe.Pointer(&reflect.SliceHeader{
		Data: sliceHeader.Data + uintptr(sliceHeader.Len-n),
		Len:  n,
		Cap:  n,
	}))
}

func GetSliceChunk[T any](data *[]T, start, end int) *[]T {
	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(data))
	return (*[]T)(unsafe.Pointer(&reflect.SliceHeader{
		Data: sliceHeader.Data + uintptr(start),
		Len:  end - start,
		Cap:  end - start,
	}))
}

//CheckBinaryPack: バイト配列が特定のフォーマットに従っているかを確認する関数。
func CheckBinaryPack(data []byte) (byte, byte, bool) {
	if len(data) >= 8 {
		if bytes.Equal(data[:4], []byte{34, 22, 19, 17}) {
			if data[4] == 20 || data[4] == 21 {
				return data[4], data[5], true
			}
		}
	}
	return 0, 0, false
}
