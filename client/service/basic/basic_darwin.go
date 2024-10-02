//go:build darwin
// +build darwin

package basic

/*
#cgo LDFLAGS: -framework CoreServices -framework Carbon
#include <CoreServices/CoreServices.h>
#include <Carbon/Carbon.h>

static OSStatus SendAppleEventToSystemProcess(AEEventID EventToSend);

OSStatus SendAppleEventToSystemProcess(AEEventID EventToSend)
{
    AEAddressDesc targetDesc;
    static const ProcessSerialNumber kPSNOfSystemProcess = { 0, kSystemProcess };
    AppleEvent eventReply = {typeNull, NULL};
    AppleEvent appleEventToSend = {typeNull, NULL};

    OSStatus error = noErr;

    error = AECreateDesc(typeProcessSerialNumber, &kPSNOfSystemProcess, sizeof(kPSNOfSystemProcess), &targetDesc);

    if (error != noErr) {
        return(error);
    }

    error = AECreateAppleEvent(kCoreEventClass, EventToSend, &targetDesc, kAutoGenerateReturnID, kAnyTransactionID, &appleEventToSend);

    AEDisposeDesc(&targetDesc);
    if (error != noErr) {
        return(error);
    }

    error = AESend(&appleEventToSend, &eventReply, kAENoReply, kAENormalPriority, kAEDefaultTimeout, NULL, NULL);

    AEDisposeDesc(&appleEventToSend);
    if (error != noErr) {
        return(error);
    }

    AEDisposeDesc(&eventReply);

    return(error);
}
*/

/*
#cgo LDFLAGS: macOSのフレームワーク（CoreServices と Carbon）をリンクするためのフラグ。これにより、C言語で書かれたmacOS APIが利用可能になります。
import "C": Goのコード内でCの関数や定数を使うための指令です。Goの cgo 機能を使って、Cで定義された関数を呼び出します。
*/
import "C"

import (
	"errors"
	"os/exec"
)

/*
macOS（Darwin OS）向けに基本的なシステム操作（ログオフ、スリープ、再起動、シャットダウン）を実行するためのGoコードです。C言語で記述されたmacOSのシステムAPIを呼び出し、Goから直接実行するためにGoとCを連携させています。

macOSのシステム操作:
macOSでは、システム操作（ログオフ、スリープ、再起動、シャットダウン）をAppleEvent APIを通じて実行できます。このAPIを使用して、特定のイベントをシステムプロセスに送信し、対応するアクションを実行します。

macOSのシステム機能を利用して基本的なシステム操作を実行するGoプログラムです。C言語で書かれたmacOSのAppleEvent APIを利用して、ログオフ、再起動、シャットダウンなどを実現しています。
*/

// I'm not familiar with macOS, that's all I can do.
func init() {
}

/*
C関数: SendAppleEventToSystemProcess

目的: 指定したAppleEvent（イベントIDを通じて定義）をシステムプロセスに送信します。
フロー:
AECreateDesc 関数でシステムプロセス（kSystemProcess）をターゲットとして指定します。
AECreateAppleEvent 関数で、送信するAppleイベントを作成します。例えば、kAESleep でスリープを要求するイベントが作成されます。
AESend 関数で実際にイベントをシステムプロセスに送信します。
送信後、イベントオブジェクトを解放します（AEDisposeDesc）。
*/

//Lock(): 現在、ロック機能はサポートされていないため、エラーメッセージが返されます。
func Lock() error {
	return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
}

/*
目的: macOSでユーザーをログオフさせる。
動作: Cの SendAppleEventToSystemProcess を呼び出し、kAEReallyLogOut イベントをシステムプロセスに送信します。送信が成功すると nil を返し、失敗するとエラーメッセージを返します。
*/
func Logoff() error {
	if C.SendAppleEventToSystemProcess(C.kAEReallyLogOut) == C.noErr {
		return nil
	} else {
		return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
	}
}

/*
目的: macOSをスリープ状態にします。
動作: Cの SendAppleEventToSystemProcess で kAESleep イベントを送信します。成功すると nil、失敗するとエラーメッセージを返します。
*/
func Hibernate() error {
	if C.SendAppleEventToSystemProcess(C.kAESleep) == C.noErr {
		return nil
	} else {
		return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
	}
}

//Suspend(): サスペンド（中断）もサポートされておらず、エラーメッセージを返します。
func Suspend() error {
	return errors.New(`${i18n|COMMON.OPERATION_NOT_SUPPORTED}`)
}

/*
目的: macOSを再起動します。
動作:
Cの SendAppleEventToSystemProcess を呼び出し、kAERestart イベントを送信します。
もしイベント送信が失敗した場合、システムの reboot コマンドを実行して再起動を試みます。
*/
func Restart() error {
	if C.SendAppleEventToSystemProcess(C.kAERestart) == C.noErr {
		return nil
	} else {
		return exec.Command(`reboot`).Run()
	}
}

/*
目的: macOSをシャットダウンします。
動作:
Cの SendAppleEventToSystemProcess を呼び出し、kAEShutDown イベントを送信します。
イベント送信に失敗した場合は、システムの shutdown コマンドを実行してシャットダウンを試みます。
*/
func Shutdown() error {
	if C.SendAppleEventToSystemProcess(C.kAEShutDown) == C.noErr {
		return nil
	} else {
		return exec.Command(`shutdown`).Run()
	}
}
