//go:build linux
// +build linux

package basic

/*
Linux向けに基本的なシステム操作（再起動、シャットダウン、ハイバネート、サスペンド）を実行するGoのコードです。syscall パッケージを使用して、Linuxカーネルのシステムコール（syscall.Syscall や syscall.Reboot）を呼び出し、システム操作を実行します。

syscall パッケージ
syscall は、Go言語で低レベルのシステムコールを呼び出すためのパッケージです。Linuxシステムで直接カーネルに対して命令を送るために使われます。
syscall.Syscall: システムコールを直接実行します。
syscall.Reboot: 再起動やシャットダウン、サスペンド、ハイバネートなどの機能を提供するシステムコールです。
エラーハンドリング
各関数では、システムコールの実行結果をエラーとして返す構造になっています。システムコールが成功すれば nil が返り、失敗すればエラーメッセージが返されます。

このコードは、Linuxカーネルのシステムコールを使用して、再起動、シャットダウン、サスペンド、ハイバネートなどの基本的なシステム操作をGoから実行できるようにしています。ただし、画面ロックやログオフといった操作はこのコードではサポートされていません。
*/

import (
	"errors"
	"syscall"
)

func init() {
}

//目的: システムをロックする（画面ロックなど）操作を実装するための関数。
//実装: 現在、ロック機能はサポートされていないため、エラーメッセージが返されます。
func Lock() error {
	return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
}

//目的: ユーザーをログオフさせるための関数。
//実装: ログオフもサポートされていないため、エラーメッセージが返されます。
func Logoff() error {
	return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
}

//目的: Linuxシステムをハイバネート状態にします（システムの状態をディスクに保存して電源をオフにする）。
//実装: syscall.Syscall を使って SYS_REBOOT を呼び出し、LINUX_REBOOT_CMD_HALT を使用してシステムをハイバネート状態にします。syscall.Syscall は低レベルのシステムコールで、Linuxカーネルの機能に直接アクセスします。
func Hibernate() error {
	// Prevent constant overflow when GOARCH is arm or i386.
	_, _, err := syscall.Syscall(syscall.SYS_REBOOT, syscall.LINUX_REBOOT_CMD_HALT, 0, 0)
	return err
}

//目的: Linuxシステムをサスペンド状態にします（電力消費を抑えるために一時的に動作を停止させる）。
//実装: 同じく syscall.Syscall を使用し、LINUX_REBOOT_CMD_SW_SUSPEND を指定してシステムをサスペンド状態にします。
func Suspend() error {
	// Prevent constant overflow when GOARCH is arm or i386.
	_, _, err := syscall.Syscall(syscall.SYS_REBOOT, syscall.LINUX_REBOOT_CMD_SW_SUSPEND, 0, 0)
	return err
}

//目的: システムを再起動します。
//実装: syscall.Reboot を呼び出し、LINUX_REBOOT_CMD_RESTART を指定して再起動を実行します。これは、Linuxシステムを安全に再起動する標準的な方法です。
func Restart() error {
	return syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
}

//目的: システムをシャットダウンします。
//実装: syscall.Reboot を使って、LINUX_REBOOT_CMD_POWER_OFF を指定し、システムをシャットダウンします。
func Shutdown() error {
	return syscall.Reboot(syscall.LINUX_REBOOT_CMD_POWER_OFF)
}
