//go:build windows
// +build windows

package basic

import (
	"syscall"
	"unsafe"
)

/*
Windows環境において、システム操作（ロック、ログオフ、ハイバネート、サスペンド、再起動、シャットダウン）を行うためのGoプログラムです。syscallパッケージを使用して、Windows APIを呼び出し、これらのシステム機能を実現しています。コードには、システム権限の取得も含まれており、Windowsの特定の操作（再起動やシャットダウンなど）を実行するために必要な特権を設定しています。


DLL呼び出しの解説
syscall.MustLoadDLL(): 指定されたDLLをロードし、GoからそのDLL内の関数を呼び出せるようにします。例えば、user32.dllやpowrprof.dllをロードします。
MustFindProc(): 指定されたDLL内のプロシージャ（関数）を検索し、そのアドレスを取得します。
Call(): 検索した関数を呼び出します。この際に必要な引数を渡して、Windows APIを実行します。


特権操作
privilege() 関数は、システムのシャットダウンや再起動などの操作を行う際に必要な特権をプロセスに付与する処理を行います。Windowsでは通常ユーザーにはこれらの特権がないため、明示的にこれを設定しなければならない場合があります。

エラーハンドリング
各操作では、Windows APIが成功したかどうかをsyscall.Errno(0)で確認し、エラーの場合にはエラーメッセージを返しています。

このコード全体は、Windows上でのシステム操作をGoプログラム内から直接実行するためのものです。
*/

func init() {
	privilege()
}

/*
privilege() 関数

役割: Windowsのシステム操作（シャットダウンや再起動など）に必要な特権（SeShutdownPrivilege）をプロセスに付与します。
詳細:
OpenProcessToken 関数を使用して、現在のプロセスのトークンを取得します。
LookupPrivilegeValue 関数を使用して、SeShutdownPrivilege の特権値を取得します。
AdjustTokenPrivileges 関数を使い、その特権をプロセスに設定します。
*/
func privilege() error {
	user32 := syscall.MustLoadDLL("user32")
	defer user32.Release()
	kernel32 := syscall.MustLoadDLL("kernel32")
	defer user32.Release()
	advapi32 := syscall.MustLoadDLL("advapi32")
	defer advapi32.Release()

	GetLastError := kernel32.MustFindProc("GetLastError")
	GetCurrentProcess := kernel32.MustFindProc("GetCurrentProcess")
	OpenProdcessToken := advapi32.MustFindProc("OpenProcessToken")
	LookupPrivilegeValue := advapi32.MustFindProc("LookupPrivilegeValueW")
	AdjustTokenPrivileges := advapi32.MustFindProc("AdjustTokenPrivileges")

	currentProcess, _, _ := GetCurrentProcess.Call()

	const tokenAdjustPrivileges = 0x0020
	const tokenQuery = 0x0008
	var hToken uintptr

	result, _, err := OpenProdcessToken.Call(currentProcess, tokenAdjustPrivileges|tokenQuery, uintptr(unsafe.Pointer(&hToken)))
	if result != 1 {
		return err
	}

	const SeShutdownName = "SeShutdownPrivilege"

	type Luid struct {
		lowPart  uint32 // DWORD
		highPart int32  // long
	}
	type LuidAndAttributes struct {
		luid       Luid   // LUID
		attributes uint32 // DWORD
	}

	type TokenPrivileges struct {
		privilegeCount uint32 // DWORD
		privileges     [1]LuidAndAttributes
	}

	var tkp TokenPrivileges

	utf16ptr, err := syscall.UTF16PtrFromString(SeShutdownName)
	if err != nil {
		return err
	}

	result, _, err = LookupPrivilegeValue.Call(uintptr(0), uintptr(unsafe.Pointer(utf16ptr)), uintptr(unsafe.Pointer(&(tkp.privileges[0].luid))))
	if result != 1 {
		return err
	}

	const SePrivilegeEnabled uint32 = 0x00000002

	tkp.privilegeCount = 1
	tkp.privileges[0].attributes = SePrivilegeEnabled

	result, _, err = AdjustTokenPrivileges.Call(hToken, 0, uintptr(unsafe.Pointer(&tkp)), 0, uintptr(0), 0)
	if result != 1 {
		return err
	}

	result, _, _ = GetLastError.Call()
	if result != 0 {
		return err
	}

	return nil
}

/*
役割: システムの画面をロックします。
詳細: user32.dll の LockWorkStation 関数を呼び出して、システムのロックを実行します。
*/
func Lock() error {
	dll := syscall.MustLoadDLL(`user32`)
	_, _, err := dll.MustFindProc(`LockWorkStation`).Call()
	dll.Release()
	if err == syscall.Errno(0) {
		return nil
	}
	return err
}

/*
役割: 現在のユーザーをログオフします。
詳細: ExitWindowsEx 関数に EWX_LOGOFF フラグを渡して、ログオフ操作を実行します。
*/
func Logoff() error {
	const EWX_LOGOFF = 0x00000000
	dll := syscall.MustLoadDLL(`user32`)
	_, _, err := dll.MustFindProc(`ExitWindowsEx`).Call(EWX_LOGOFF, 0x0)
	dll.Release()
	if err == syscall.Errno(0) {
		return nil
	}
	return err
}

/*
役割: システムをハイバネート（休止状態）にします。
詳細: powrprof.dll の SetSuspendState 関数を呼び出して、ハイバネートを実行します。
*/
func Hibernate() error {
	const HIBERNATE = 0x00000001
	dll := syscall.MustLoadDLL(`powrprof`)
	_, _, err := dll.MustFindProc(`SetSuspendState`).Call(HIBERNATE, 0x0, 0x1)
	dll.Release()
	if err == syscall.Errno(0) {
		return nil
	}
	return err
}

/*
役割: システムをサスペンド（スリープ状態）にします。
詳細: SetSuspendState 関数を呼び出して、サスペンドを実行します。
*/
func Suspend() error {
	const SUSPEND = 0x00000000
	dll := syscall.MustLoadDLL(`powrprof`)
	_, _, err := dll.MustFindProc(`SetSuspendState`).Call(SUSPEND, 0x0, 0x1)
	dll.Release()
	if err == syscall.Errno(0) {
		return nil
	}
	return err
}

/*
役割: システムを再起動します。
詳細: ExitWindowsEx 関数に EWX_REBOOT | EWX_FORCE フラグを渡して、強制再起動を実行します。
*/
func Restart() error {
	const EWX_REBOOT = 0x00000002
	const EWX_FORCE = 0x00000004
	dll := syscall.MustLoadDLL(`user32`)
	_, _, err := dll.MustFindProc(`ExitWindowsEx`).Call(EWX_REBOOT|EWX_FORCE, 0x0)
	dll.Release()
	if err == syscall.Errno(0) {
		return nil
	}
	return err
}

/*
役割: システムをシャットダウンします。
詳細: ExitWindowsEx 関数に EWX_SHUTDOWN | EWX_FORCE フラグを渡して、強制シャットダウンを実行します。
*/
func Shutdown() error {
	const EWX_SHUTDOWN = 0x00000001
	const EWX_FORCE = 0x00000004
	dll := syscall.MustLoadDLL(`user32`)
	_, _, err := dll.MustFindProc(`ExitWindowsEx`).Call(EWX_SHUTDOWN|EWX_FORCE, 0x0)
	dll.Release()
	if err == syscall.Errno(0) {
		return nil
	}
	return err
}
