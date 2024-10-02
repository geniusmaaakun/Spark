//go:build !windows
// +build !windows

package desktop

import (
	"image"

	"github.com/kbinani/screenshot"
)

/*
Windows以外のOS（LinuxやmacOSなど）でスクリーンキャプチャ（画面の一部または全体を画像として取得）を行うためのGoプログラムです。github.com/kbinani/screenshot パッケージを利用して、指定された範囲（rect）のスクリーンショットをキャプチャします。


目的: このコードは、Windows以外のプラットフォームで、指定された領域のスクリーンショットを取得するために使用されます。
使用するパッケージ: github.com/kbinani/screenshot パッケージを使い、スクリーンキャプチャの処理を簡単に実装しています。
役割: Screen 構造体を使ってキャプチャ領域を指定し、その領域のスクリーンショットを取得できるようにしています。
*/

/*
役割: スクリーンキャプチャのための情報を管理します。
フィールド:
rect: image.Rectangle 型で、キャプチャする画面の領域（四角形の範囲）を指定します。この矩形は、キャプチャする範囲の左上と右下の座標を持ちます。
*/
type Screen struct {
	rect image.Rectangle
}

/*
役割: スクリーンキャプチャを行う範囲（矩形）を初期化します。
引数:
_ uint: 使用されない引数です。ここでは無視されます。
rect: image.Rectangle 型で、スクリーンキャプチャする範囲の矩形を指定します。この矩形を s.rect フィールドに保存します。
用途: キャプチャしたい範囲を定義します。
*/
func (s *Screen) Init(_ uint, rect image.Rectangle) {
	s.rect = rect
}

/*
役割: s.rect で定義された範囲のスクリーンショットをキャプチャします。
戻り値:
*image.RGBA: キャプチャされた画像が RGBA 形式で返されます。これはキャプチャされたスクリーンショットです。
error: キャプチャに失敗した場合のエラー情報が返されます。
詳細: screenshot.CaptureRect(s.rect) 関数を使用して、指定した範囲（s.rect）をキャプチャします。
*/
func (s *Screen) Capture() (*image.RGBA, error) {
	return screenshot.CaptureRect(s.rect)
}

/*
役割: リソースの解放を行うためのメソッドですが、この場合は何も行っていません。
詳細: Release() メソッドは、オブジェクトやリソースの解放処理を記述するために使われることが多いですが、このコードでは特にリソースを解放する必要がないため、何も処理を行いません。
*/
func (s *Screen) Release() {}
