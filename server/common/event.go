package common

import (
	"Spark/modules"
	"Spark/utils/cmap"
	"Spark/utils/melody"
	"time"
)

/*
イベントベースのコールバック管理を行うための機能を提供しています。
具体的には、特定のイベントが発生した際に、そのイベントに紐付けられたコールバック関数を実行する仕組みを実現しています。イベントは一度だけ実行されるもの（AddEventOnce）と、繰り返し実行可能なもの（AddEvent）の2種類があります。


使い方の例
一度だけ発生するイベントの追加

go
コードをコピーする
AddEventOnce(func(pack modules.Packet, session *melody.Session) {
    fmt.Println("Received event:", pack)
}, "session-uuid", "unique-event-trigger", 10*time.Second)
このイベントは10秒以内に発生しなければ削除されます。発生すればコールバック関数が実行されます。
繰り返し発生するイベントの追加

go
コードをコピーする
AddEvent(func(pack modules.Packet, session *melody.Session) {
    fmt.Println("Event triggered:", pack)
}, "session-uuid", "repeating-event-trigger")
イベントの呼び出し

go
コードをコピーする
CallEvent(pack, session)
pack.Eventに基づいてイベントがトリガーされ、対応するコールバック関数が呼び出されます。
イベントの削除

go
コードをコピーする
RemoveEvent("unique-event-trigger", true)


イベントベースの非同期コールバックシステムを提供しています。特定のトリガーを使用してイベントを登録し、イベントが発生した際にコールバック関数を呼び出します。また、タイムアウトやイベントの一度限りの実行などもサポートしています。
*/

type EventCallback func(modules.Packet, *melody.Session)

/*
connection: イベントが発生したときに特定のWebSocketセッションに関連付けられるUUID（connUUID）です。
callback: イベントが発生したときに実行されるコールバック関数（EventCallback）です。コールバック関数の引数としてmodules.Packetとセッション*melody.Sessionが渡されます。
finish: イベントが完了したときに通知するチャネル。主にAddEventOnceで使われます。
remove: イベントが削除されるときに通知するチャネルです。
*/
type event struct {
	connection string
	callback   EventCallback
	finish     chan bool
	remove     chan bool
}

/*
events: スレッドセーフなマップで、イベントをトリガー（trigger）ごとに管理します。cmapはスレッドセーフなマップ実装を使用しており、複数のゴルーチンから同時にアクセスされても安全に動作します。
*/
var events = cmap.New[*event]()

/*
**CallEvent**は、特定のイベントをトリガーし、そのイベントに紐付けられたコールバック関数を実行します。
pack.Eventが存在するか確認し、イベントが登録されていれば取得します（events.Get）。
セッション（session）が関連付けられている場合、該当するセッションでないとイベントは実行されません。
イベントが実行されたら、ev.finishチャネルにtrueを送信します。これはAddEventOnceの終了処理を通知するために使われます。
*/
// CallEvent tries to call the callback with the given uuid
// after that, it will notify the caller via the channel
func CallEvent(pack modules.Packet, session *melody.Session) {
	if len(pack.Event) == 0 {
		return
	}
	ev, ok := events.Get(pack.Event)
	if !ok {
		return
	}
	if session != nil && session.UUID != ev.connection {
		return
	}
	ev.callback(pack, session)
	if ev.finish != nil {
		ev.finish <- true
	}
}

/*
**AddEventOnce**は、一度だけ呼び出されるイベントを追加します。このイベントは、指定されたtriggerが発生するか、タイムアウトするまで待機します。
イベントが完了（finish）または削除（remove）されたら、イベントを削除して結果を返します。
タイムアウトが発生した場合もイベントは削除され、falseを返します。
*/
// AddEventOnce adds a new event only once and client
// can call back the event with the given event trigger.
// Event trigger should be uuid to make every event unique.
func AddEventOnce(fn EventCallback, connUUID, trigger string, timeout time.Duration) bool {
	ev := &event{
		connection: connUUID,
		callback:   fn,
		finish:     make(chan bool),
		remove:     make(chan bool),
	}
	events.Set(trigger, ev)
	defer close(ev.remove)
	defer close(ev.finish)
	select {
	case ok := <-ev.finish:
		events.Remove(trigger)
		return ok
	case ok := <-ev.remove:
		events.Remove(trigger)
		return ok
	case <-time.After(timeout):
		events.Remove(trigger)
		return false
	}
}

//*AddEvent**は、繰り返し呼び出せるイベントを追加します。AddEventOnceと違って、一度呼ばれてもそのまま残り続けます。
// AddEvent adds a new event and client can call back
// the event with the given event trigger.
func AddEvent(fn EventCallback, connUUID, trigger string) {
	ev := &event{
		connection: connUUID,
		callback:   fn,
	}
	events.Set(trigger, ev)
}

//**RemoveEvent**は、指定されたtriggerに関連付けられたイベントを削除します。ok引数を渡すことで、削除時に特定のステータスを設定できます（trueやfalseを指定可能）。
// 削除された後、関連付けられたev.removeチャネルに通知が送信されます。
// RemoveEvent deletes the event with the given event trigger.
// The ok will be returned to caller if the event is temp (only once).
func RemoveEvent(trigger string, ok ...bool) {
	ev, found := events.Get(trigger)
	if !found {
		return
	}
	events.Remove(trigger)
	if ev.remove != nil {
		if len(ok) > 0 {
			ev.remove <- ok[0]
		} else {
			ev.remove <- false
		}
	}
	ev = nil
}

//**HasEvent**は、指定されたtriggerが存在するかどうかを確認する関数です。イベントが存在すればtrueを返します。
// HasEvent returns if the event exists.
func HasEvent(trigger string) bool {
	return events.Has(trigger)
}
