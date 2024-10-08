package terminal

import (
	"errors"
)

/*
リモート操作で使われる仮想端末セッションのプロトコルに関する定義とエラーハンドリングを提供しています。仮想端末セッションでやり取りされるパケットのフォーマットとエラーに関連する情報を定義しています。


1. エラーハンドリング
コードにはいくつかのエラーが定義されています。これらのエラーは、仮想端末セッションにおいて不正なデータや識別子が見つからなかった場合に使用されます。

errDataNotFound: パケットにデータが見つからない場合に発生するエラー。
errDataInvalid: パケットのデータが解析できない場合に発生するエラー。
errUUIDNotFound: ターミナルの識別子（UUID）が見つからない場合に発生するエラー。
2. パケットフォーマット
パケットフォーマットは仮想端末セッションにおいてデータの送受信に使用されます。このフォーマットは、通信プロトコルの一部であり、仮想端末のデータを送信する際の構造が定義されています。

パケット構造
magic (5 bytes): パケットの開始部分を示す定数。値は []byte{34, 22, 19, 17, 21} です。通信の開始を識別するために使用されます。
op code (1 byte): パケットの操作コードを示す1バイトのフィールド。操作の種類を示します。
00: バイナリデータパケット（実際の端末の出力などが含まれる）。
01: JSONデータパケット（設定やメタ情報などが含まれる）。
event id (16 bytes): 16バイトのイベント識別子。セッションやイベントを特定するためのユニークIDが入ります。
data length (2 bytes): データの長さを示す2バイトのフィールド。パケットの中のデータのサイズを表します。
data: 実際に送信されるデータ。長さは data length によって決まります。
まとめ
このコードは、仮想端末セッションにおけるデータのやり取りを管理するためのエラーハンドリングとパケットのフォーマット定義を提供しています。エラーは、データが見つからない、解析できない、もしくはUUIDが見つからない場合に使用されます。また、パケットフォーマットは通信プロトコルの詳細な構造を定義し、仮想端末のデータ送受信の仕組みを示しています。

*/

var (
	errDataNotFound = errors.New(`no input found in packet`)
	errDataInvalid  = errors.New(`can not parse data in packet`)
	errUUIDNotFound = errors.New(`can not find terminal identifier`)
)

// packet explanation:

// +---------+---------+----------+-------------+------+
// | magic   | op code | event id | data length | data |
// +---------+---------+----------+-------------+------+
// | 5 bytes | 1 byte  | 16 bytes | 2 bytes     | -    |
// +---------+---------+----------+-------------+------+

// magic:
// []byte{34, 22, 19, 17, 21}

// op code:
// 00: binary packet
// 01: JSON packet
