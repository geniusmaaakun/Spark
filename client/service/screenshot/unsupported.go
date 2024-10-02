//go:build !linux && !windows && !darwin

package screenshot

/*
Linux、Windows、macOS以外のプラットフォームでビルドされた場合に、スクリーンショット機能がサポートされていないことを示すためのエラーハンドリングを提供しています。
*/

import "errors"

/*
ビルドタグ (//go:build !linux && !windows && !darwin)
このビルドタグは、このコードが Linux、Windows、macOS (Darwin) 以外のプラットフォーム（例えばFreeBSD、Solarisなど）でのみコンパイルされることを指定しています。
!linux && !windows && !darwin は、「Linux、Windows、またはmacOSでない」環境という条件を意味します。
2. GetScreenshot 関数
この関数は、他のプラットフォーム用に定義されている GetScreenshot 関数と同じシグネチャを持っていますが、内容が異なります。
関数は、引数として bridge（他のプラットフォームで使われているパラメータ）を受け取りますが、スクリーンショット機能がサポートされていないため、何も行いません。
3. エラーハンドリング
errors.New('${i18n|COMMON.OPERATION_NOT_SUPPORTED}') でエラーメッセージを返します。メッセージの内容は国際化対応用のプレースホルダーであり、適切なエラーメッセージに置き換わることを想定しています（例: "Operation not supported"）。
このエラーは「操作がサポートされていない」という意味で、スクリーンショット機能がこのプラットフォームでは提供されていないことを明示的に通知します。
まとめ
非対応プラットフォーム向けの実装: Linux、Windows、macOS以外のプラットフォームではスクリーンショット機能を提供していないため、この関数はサポートされていない旨のエラーメッセージを返します。
国際化対応: エラーメッセージは ${i18n|COMMON.OPERATION_NOT_SUPPORTED} というプレースホルダーを使用しており、異なる言語に対応できるようになっています。
このコードは、プラットフォーム間の互換性を保つための方法として、ビルドタグを利用して動作しないプラットフォームで適切にエラーを返す処理を行っています。
*/
func GetScreenshot(bridge string) error {
	return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
}
