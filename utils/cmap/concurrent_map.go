package cmap

import (
	"encoding/json"
	"fmt"
	"sync"
)

/*
スレッドセーフなマップ（ConcurrentMap）を実装するものです。通常のマップはスレッドセーフではないため、複数のゴルーチンから同時にアクセスすると問題が発生します。この実装では、マップを複数のシャード（分割された部分）に分けることで、ロックの競合を最小限に抑えつつ、スレッドセーフに動作させています。
*/

/*
シャード（shard）とは、大規模なデータを管理する際に、1つの大きなデータ構造を複数の小さな部分（シャード）に分割して扱う設計パターンです。この分割により、データを並列に処理することができ、スケーラビリティやパフォーマンスの向上を目指します。

特に、シャーディング（sharding）は以下のようなシステムや状況で使われます。

1. データベースにおけるシャード
大量のデータを扱うデータベースでは、全てのデータを1つのサーバーで処理すると処理速度や容量の限界があります。そのため、データを複数のサーバーに分割して保存します。
各サーバーに格納されているデータの部分をシャードと呼びます。この方式では、例えばユーザーIDの範囲や地理的な情報をもとにデータをシャードに分けることが多いです。
2. キャッシュやキーバリューストアでのシャード
RedisやMemcachedなどのキーバリューストアやキャッシュシステムでも、シャーディングが使われます。
データ（キーと値のペア）が増えると、1つのノードで全てを管理するのは困難になります。キーのハッシュ値などに基づいて、データを複数のノード（シャード）に分散して保存することで、負荷を分散させ、システムの処理性能を向上させます。
3. 並行処理におけるシャード
ConcurrentMapの例のように、データ構造を複数のシャードに分割してスレッド間で競合が起こるのを防ぎます。
通常のマップは1つの大きなデータ構造であり、複数のゴルーチン（スレッド）から同時にアクセスされると、データが壊れる可能性があるためロック（sync.Mutexなど）を使って競合を防ぎます。しかし、全体にロックをかけるとパフォーマンスに悪影響が出ます。
シャーディングでは、データ構造を複数の部分に分割し、各部分（シャード）に対して個別にロックをかけます。これにより、複数のスレッドが同時に異なるシャードにアクセスできるため、並行処理の効率が向上します。
4. シャーディングのメリット
スケーラビリティ: データが分割されているため、サーバーやストレージの増加によって簡単に処理能力を拡張できます。
パフォーマンス向上: 複数のシャードに分けることで、同時にデータを処理できるため、処理速度が向上します。
負荷分散: データが分割されていることで、特定のサーバーやプロセスに負荷が集中するのを防げます。
5. シャーディングの実装例（ハッシュベースのシャーディング）
シャードをどのように分けるかを決定する際、キーに対してハッシュ関数を使うことが一般的です。例えば、fnv32などのハッシュ関数を使って、キーを特定のシャードに割り振ります。
キーのハッシュ値をシャード数で割った余りを使って、どのシャードにデータを保存するかを決定します。
これにより、データが均等にシャードに分散され、効率的に管理できます。
まとめ
シャードは、データや処理を複数の小さな単位に分割する技術で、システムのスケーラビリティやパフォーマンスを向上させるために使われます。
特に大規模なデータベース、キャッシュ、並行処理が必要なシステムで使われる設計パターンで、負荷分散やロック競合を回避する手段として効果的です。
*/

// 32で割る
var SHARD_COUNT = 32

type Stringer interface {
	fmt.Stringer
	comparable
}

// key, value
// **ConcurrentMap**は、キーKと値Vを持つスレッドセーフなマップです。
// shards: マップを分割した個々の部分を表すConcurrentMapSharedの配列です。スレッド間の競合を避けるため、マップ全体をシャードに分割しています。
// sharding: キーKに基づいてシャードを選ぶためのハッシュ関数です。この関数を使って、特定のキーがどのシャードに対応するかを決定します。
// A "thread" safe map of type string:Anything.
// To avoid lock bottlenecks this map is dived to several (SHARD_COUNT) map shards.
type ConcurrentMap[K comparable, V any] struct {
	// 複数に分割するためのシャードの配列
	shards []*ConcurrentMapShared[K, V]
	// シャードを選択するためのハッシュ関数
	sharding func(key K) uint32
}

// **ConcurrentMapShared**は、個々のシャードを表します。
// このシャード自体は通常のGoのマップですが、スレッドセーフに操作するために読み書きのロック（sync.RWMutex）が使用されています。
// A "thread" safe string to anything map.
type ConcurrentMapShared[K comparable, V any] struct {
	items        map[K]V
	sync.RWMutex // Read Write mutex, guards access to internal map.
}

// マップを初期化する
func create[K comparable, V any](sharding func(key K) uint32) ConcurrentMap[K, V] {
	m := ConcurrentMap[K, V]{
		sharding: sharding,
		shards:   make([]*ConcurrentMapShared[K, V], SHARD_COUNT),
	}
	// 配列の中の要素を初期化
	for i := 0; i < SHARD_COUNT; i++ {
		m.shards[i] = &ConcurrentMapShared[K, V]{items: make(map[K]V)}
	}
	return m
}

// この関数は、キーがstring型で、値がV型（任意の型）のConcurrentMapを作成します。
// fnv32というハッシュ関数を使って、キーのstringを32ビットのハッシュ値に変換します。これにより、キーに基づいてシャードを選択します。
// Creates a new concurrent map.
func New[V any]() ConcurrentMap[string, V] {
	return create[string, V](fnv32)
}

// Creates a new concurrent map.
// この部分は K 型を元にしてハッシュ値を生成する関数（strfnv32）を呼び出しています。
func NewStringer[K Stringer, V any]() ConcurrentMap[K, V] {
	return create[K, V](strfnv32[K])
}

// Creates a new concurrent map.
// NewWithCustomShardingFunction 関数は、任意のキー型 K とカスタムシャーディング関数（ハッシュ関数）を使用して、スレッドセーフなマップを作成します。
func NewWithCustomShardingFunction[K comparable, V any](sharding func(key K) uint32) ConcurrentMap[K, V] {
	//カスタムシャーディング関数 customHash を指定してマップを作成します。
	return create[K, V](sharding)
}

// GetShardは、指定されたキーkeyに基づいて、そのキーが属するシャードを返します。
// sharding関数によってキーのハッシュ値を計算し、シャードの数SHARD_COUNTで割った余りを使ってシャードを決定します。
// GetShard returns shard under given key
func (m ConcurrentMap[K, V]) GetShard(key K) *ConcurrentMapShared[K, V] {
	return m.shards[uint(m.sharding(key))%uint(SHARD_COUNT)]
}

// mapをシャードに格納
func (m ConcurrentMap[K, V]) MSet(data map[K]V) {
	for key, value := range data {
		shard := m.GetShard(key)
		shard.Lock()
		shard.items[key] = value
		shard.Unlock()
	}
}

/*
このメソッドは、指定されたキーkeyに対して値valueを設定します。
GetShardを使って該当するシャードを取得し、そのシャードに対して書き込みを行います。書き込み時にはロック（Lock）を取得し、データの一貫性を保証します。
*/
// Sets the given value under the specified key.
func (m ConcurrentMap[K, V]) Set(key K, value V) {
	// Get map shard.
	shard := m.GetShard(key)
	shard.Lock()
	shard.items[key] = value
	shard.Unlock()
}

// この型は、並行マップ（スレッドセーフなマップ）で値を挿入または更新する際の動作を指定するためのものです。
// V はマップの値の型を表します。
/*
関数の引数:

exist bool:
指定したキーがマップ内にすでに存在している場合は true、存在しない場合は false。
これを使って、新しい要素を挿入するか、既存の要素を更新するかを判断します。
valueInMap V:
キーがすでに存在している場合、現在マップに格納されている値です。
キーが存在しない場合は、デフォルト値（型 V のゼロ値）が渡されることが一般的です。
newValue V:
挿入または更新したい新しい値です。
戻り値:

V:
このコールバックが返す値が最終的にマップに格納されます。
*/
// Callback to return new element to be inserted into the map
// It is called while lock is held, therefore it MUST NOT
// try to access other keys in same map, as it can lead to deadlock since
// Go sync.RWLock is not reentrant
type UpsertCb[V any] func(exist bool, valueInMap V, newValue V) V

// Upsertは、キーkeyが既に存在する場合は更新し、存在しない場合は新規に挿入します。
// コールバック関数UpsertCbを使用して、既存の値と新しい値をマージするなど、柔軟な挙動が可能です。
// Insert or Update - updates existing element or inserts a new one using UpsertCb
func (m ConcurrentMap[K, V]) Upsert(key K, value V, cb UpsertCb[V]) (res V) {
	shard := m.GetShard(key)
	shard.Lock()
	v, ok := shard.items[key]
	res = cb(ok, v, value)
	shard.items[key] = res
	shard.Unlock()
	return res
}

// SetIfAbsent で、並行（スレッドセーフ）マップに対して「指定されたキーが存在しない場合に値を設定する」動作を実現します。
// Sets the given value under the specified key if no value was associated with it.
func (m ConcurrentMap[K, V]) SetIfAbsent(key K, value V) bool {
	// Get map shard.
	//対象のシャードを取得
	shard := m.GetShard(key)
	shard.Lock()
	//キーの存在確認
	_, ok := shard.items[key]
	//key が存在しない場合のみ、新しい値を設定します。
	if !ok {
		shard.items[key] = value
	}
	shard.Unlock()
	return !ok
}

// 指定されたキーkeyに対応する値を取得します。
// RLockで読み込み用のロックを取得してから、マップにアクセスします。
// Get retrieves an element from map under given key.
func (m ConcurrentMap[K, V]) Get(key K) (V, bool) {
	// Get shard
	shard := m.GetShard(key)
	shard.RLock()
	// Get item from shard.
	val, ok := shard.items[key]
	shard.RUnlock()
	return val, ok
}

// このメソッドは、ConcurrentMap全体の要素数を返します。
// 各シャードに対してRLockを取得し、要素数を集計します。
// Count returns the number of elements within the map.
func (m ConcurrentMap[K, V]) Count() int {
	count := 0
	for i := 0; i < SHARD_COUNT; i++ {
		shard := m.shards[i]
		shard.RLock()
		count += len(shard.items)
		shard.RUnlock()
	}
	return count
}

// keyの要素が存在するかを確認
// Looks up an item under specified key
func (m ConcurrentMap[K, V]) Has(key K) bool {
	// Get shard
	shard := m.GetShard(key)
	shard.RLock()
	// See if element is within shard.
	_, ok := shard.items[key]
	shard.RUnlock()
	return ok
}

// 指定されたキーを削除します。
// Lockを取得してから、そのシャード内のitemsマップからキーを削除します。
// Remove removes an element from the map.
func (m ConcurrentMap[K, V]) Remove(keys ...K) {
	// Try to get shard.
	for _, k := range keys {
		shard := m.GetShard(k)
		shard.Lock()
		delete(shard.items, k)
		shard.Unlock()
	}
}

// 削除関数の型
// RemoveCb is a callback executed in a map.RemoveCb() call, while Lock is held
// If returns true, the element will be removed from the map
type RemoveCb[K any, V any] func(key K, v V, exists bool) bool

// 削除関数
// RemoveCb locks the shard containing the key, retrieves its current value and calls the callback with those params
// If callback returns true and element exists, it will remove it from the map
// Returns the value returned by the callback (even if element was not present in the map)
func (m ConcurrentMap[K, V]) RemoveCb(key K, cb RemoveCb[K, V]) bool {
	// Try to get shard.
	shard := m.GetShard(key)
	shard.Lock()
	v, ok := shard.items[key]
	remove := cb(key, v, ok)
	if remove && ok {
		delete(shard.items, key)
	}
	shard.Unlock()
	return remove
}

// 削除関数
// callback関数は実行しない
// Pop removes an element from the map and returns it
func (m ConcurrentMap[K, V]) Pop(key K) (v V, exists bool) {
	// Try to get shard.
	shard := m.GetShard(key)
	shard.Lock()
	v, exists = shard.items[key]
	delete(shard.items, key)
	shard.Unlock()
	return v, exists
}

// 要素が空かどうかを確認
// IsEmpty checks if map is empty.
func (m ConcurrentMap[K, V]) IsEmpty() bool {
	return m.Count() == 0
}

// key value pair
// Used by the Iter & IterBuffered functions to wrap two variables together over a channel,
type Tuple[K comparable, V any] struct {
	Key K
	Val V
}

// ConcurrentMap 型における Iter メソッドの実装で、スレッドセーフなマップの内容を反復処理するためのイテレータ（チャネル）を提供します。
// ただし、このメソッドはパフォーマンス上の理由から 非推奨（Deprecated）とされています。
// Iter returns an iterator which could be used in a for range loop.
// Deprecated: using IterBuffered() will get a better performence
func (m ConcurrentMap[K, V]) Iter() <-chan Tuple[K, V] {
	chans := snapshot(m)
	ch := make(chan Tuple[K, V])
	go fanIn(chans, ch)
	return ch
}

// IterBufferedは、マップのすべての要素をTuple型のチャネルで返します。
// snapshot関数を使って各シャードの要素をチャネルにコピーし、その後、fanIn関数でそれらを一つのチャネルに集約します。
// IterBuffered returns a buffered iterator which could be used in a for range loop.
func (m ConcurrentMap[K, V]) IterBuffered() <-chan Tuple[K, V] {
	chans := snapshot(m)
	total := 0
	for _, c := range chans {
		total += cap(c)
	}
	ch := make(chan Tuple[K, V], total)
	go fanIn(chans, ch)
	return ch
}

// すべての要素を削除
// Clear removes all items from map.
func (m ConcurrentMap[K, V]) Clear() {
	for item := range m.IterBuffered() {
		m.Remove(item.Key)
	}
}

// ConcurrentMap に対する スナップショット（スレッドセーフな現在の状態のキャプチャ）を生成するための関数です。
// スナップショットは、ConcurrentMap の全てのシャード（分割された部分）からキーと値のペアをまとめて、各シャードに対応するチャネル（バッファ付き）に格納します。
// Returns a array of channels that contains elements in each shard,
// which likely takes a snapshot of `m`.
// It returns once the size of each buffered channel is determined,
// before all the channels are populated using goroutines.
func snapshot[K comparable, V any](m ConcurrentMap[K, V]) (chans []chan Tuple[K, V]) {
	//When you access map items before initializing.
	if len(m.shards) == 0 {
		panic(`cmap.ConcurrentMap is not initialized. Should run New() before usage.`)
	}
	chans = make([]chan Tuple[K, V], SHARD_COUNT)
	wg := sync.WaitGroup{}
	wg.Add(SHARD_COUNT)
	// Foreach shard.
	for index, shard := range m.shards {
		go func(index int, shard *ConcurrentMapShared[K, V]) {
			// Foreach key, value pair.
			shard.RLock()
			chans[index] = make(chan Tuple[K, V], len(shard.items))
			wg.Done()
			for key, val := range shard.items {
				chans[index] <- Tuple[K, V]{key, val}
			}
			shard.RUnlock()
			close(chans[index])
		}(index, shard)
	}
	wg.Wait()
	return chans
}

// 複数のチャネル（chans）からデータを一つの出力チャネル（out）にまとめる処理を行う**"fan-in"**と呼ばれる操作を実現しています。以下に詳しく解説します。
// fanIn reads elements from channels `chans` into channel `out`
func fanIn[K comparable, V any](chans []chan Tuple[K, V], out chan Tuple[K, V]) {
	wg := sync.WaitGroup{}
	wg.Add(len(chans))
	for _, ch := range chans {
		go func(ch chan Tuple[K, V]) {
			for t := range ch {
				out <- t
			}
			wg.Done()
		}(ch)
	}
	wg.Wait()
	close(out)
}

// すべてのitemsを取り出す
// Items returns all items as map[string]V
func (m ConcurrentMap[K, V]) Items() map[K]V {
	tmp := make(map[K]V)

	// Insert items to temporary map.
	for item := range m.IterBuffered() {
		tmp[item.Key] = item.Val
	}

	return tmp
}

// Iterator callbacalled for every key,value found in
// maps. RLock is held for all calls for a given shard
// therefore callback sess consistent view of a shard,
// but not across the shards
type IterCb[K comparable, V any] func(key K, v V) bool

// Callback based iterator, cheapest way to read
// all elements in a map.
func (m ConcurrentMap[K, V]) IterCb(fn IterCb[K, V]) {
	escape := false
	for idx := range m.shards {
		shard := (m.shards)[idx]
		shard.RLock()
		for key, value := range shard.items {
			if !fn(key, value) {
				escape = true
				break
			}
		}
		shard.RUnlock()
		if escape {
			break
		}
	}
}

// Keys returns all keys as []string
func (m ConcurrentMap[K, V]) Keys() []K {
	count := m.Count()
	ch := make(chan K, count)
	go func() {
		// Foreach shard.
		wg := sync.WaitGroup{}
		wg.Add(SHARD_COUNT)
		for _, shard := range m.shards {
			go func(shard *ConcurrentMapShared[K, V]) {
				// Foreach key, value pair.
				shard.RLock()
				for key := range shard.items {
					ch <- key
				}
				shard.RUnlock()
				wg.Done()
			}(shard)
		}
		wg.Wait()
		close(ch)
	}()

	// Generate keys
	keys := make([]K, 0, count)
	for k := range ch {
		keys = append(keys, k)
	}
	return keys
}

// このメソッドは、ConcurrentMapをJSONにシリアライズするためのものです。
// IterBufferedで全要素を取り出し、一時マップにコピーしてから、そのマップをJSONに変換します。
// Reviles ConcurrentMap "private" variables to json marshal.
func (m ConcurrentMap[K, V]) MarshalJSON() ([]byte, error) {
	// Create a temporary map, which will hold all item spread across shards.
	tmp := make(map[K]V)

	// Insert items to temporary map.
	for item := range m.IterBuffered() {
		tmp[item.Key] = item.Val
	}
	return json.Marshal(tmp)
}

// keyの文字列を出力する関数
func strfnv32[K fmt.Stringer](key K) uint32 {
	return fnv32(key.String())
}

// FNV-1a ハッシュアルゴリズムを用いて、文字列キー (key) を 32 ビットのハッシュ値 (uint32) に変換する関数です。このアルゴリズムは、計算が高速で、かつ一意のハッシュ値を得やすい特性を持っています。
// fnv32 関数は、与えられた文字列 key を基に、一意の 32 ビットハッシュ値を生成します。このハッシュ値は主にデータ構造（例：ハッシュテーブル）でキーの識別やデータの効率的なアクセスに使われます。
func fnv32(key string) uint32 {
	//hash: ハッシュの初期値として 32 ビットの定数 2166136261 を設定しています。この値は FNV アルゴリズムで推奨される定数です。
	//prime32: ハッシュの計算時に使う「素数」を定義しています。これも FNV アルゴリズムにおける標準の値です。
	hash := uint32(2166136261)
	const prime32 = uint32(16777619)
	//入力文字列 key の長さを計算し、ループで利用します。
	keyLength := len(key)
	for i := 0; i < keyLength; i++ {
		//hash *= prime32: 現在のハッシュ値に prime32 を掛けます。これによりハッシュ値の拡散が進みます。
		hash *= prime32
		//hash ^= uint32(key[i]): ハッシュ値と文字の ASCII コード（key[i]）を XOR 演算します。これにより文字列の各要素がハッシュ値に影響を与えるようになります。
		hash ^= uint32(key[i])
	}
	return hash
}

// Reverse process of Marshal.
func (m *ConcurrentMap[K, V]) UnmarshalJSON(b []byte) (err error) {
	tmp := make(map[K]V)

	// Unmarshal into a single map.
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	// foreach key,value pair in temporary map insert into our concurrent map.
	for key, val := range tmp {
		m.Set(key, val)
	}
	return nil
}
