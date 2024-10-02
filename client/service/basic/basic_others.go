//go:build !linux && !windows && !darwin

package basic

import (
	"errors"
	"os/exec"
)

/*
OS制御について
このコードは、ビルドタグ（//go:build !linux && !windows && !darwin）に基づいて、Linux、Windows、macOS以外のプラットフォームでのみ使用されます。つまり、BSDや他のUnix系システムなどが対象です。
*/

/*
Goで書かれた基本的なシステム操作を実装するための関数群です。特定のOS（Linux、Windows、macOS以外）の場合に実行されます。このコードは、システムの再起動やシャットダウンなどの操作を提供しますが、他の操作（ログオフ、サスペンド、ハイバネートなど）はサポートされていません。
*/

func init() {
}

/*
目的: システムの画面ロックを実行する。
実装: この関数では、画面ロック操作がサポートされていないため、エラーメッセージ ${i18n|COMMON.OPERATION_NOT_SUPPORTED} が返されます。
*/
func Lock() error {
	return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
}

/*
目的: 現在のユーザーセッションをログオフする。
実装: ログオフ操作もサポートされていないため、同様にエラーメッセージ ${i18n|COMMON.OPERATION_NOT_SUPPORTED} が返されます。
*/
func Logoff() error {
	return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
}

/*
目的: システムをハイバネート状態にする。
実装: この関数でもハイバネートはサポートされていないため、エラーメッセージが返されます。
*/
func Hibernate() error {
	return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
}

/*
目的: システムをサスペンド（スリープ）状態にする。
実装: サスペンド操作もサポートされていないため、エラーメッセージが返されます。
*/
func Suspend() error {
	return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
}

//exec.Command の役割
// exec.Command は、外部コマンド（この場合は reboot や shutdown）をGoプログラムから実行するための関数です。
// この関数は、コマンドを実行し、終了するまで待機します。実行結果が返され、エラーが発生した場合はそれが返されます。

/*
目的: システムを再起動する。
実装: exec.Command を使用して、システムの reboot コマンドを実行します。このコマンドにより、システムが再起動されます。エラーが発生した場合、そのエラーが返されます。
*/
func Restart() error {
	return exec.Command(`reboot`).Run()
}

/*
目的: システムをシャットダウンする。
実装: exec.Command を使用して、システムの shutdown コマンドを実行します。これにより、システムがシャットダウンされます。エラーが発生した場合、そのエラーが返されます。
*/
func Shutdown() error {
	return exec.Command(`shutdown`).Run()
}
