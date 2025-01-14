// React のフック (useState, useEffect, useCallback) を使用して状態管理や副作用を制御。
// 独自のユーティリティ関数をインポートして、暗号化/復号化やサイズフォーマットなどの処理を実現。
// ローカライズ機能 (i18n) を使用して多言語対応。
// DraggableModal は、ドラッグ可能なモーダルウィンドウを提供するコンポーネント。
// Ant Design ライブラリから、ボタンやアイコンをインポート。
import React, {useCallback, useEffect, useState} from 'react';
import {encrypt, decrypt, formatSize, genRandHex, getBaseURL, translate, str2ua, hex2ua, ua2hex} from "../../utils/utils";
import i18n from "../../locale/locale";
import DraggableModal from "../modal";
import {Button, message} from "antd";
import {FullscreenOutlined, ReloadOutlined} from "@ant-design/icons";


//WebSocket を利用したリアルタイムの画面共有やリモート操作システムの一部を実装するものです。Canvas API や暗号化を組み合わせてセキュアかつ効率的にデータを処理しています。

let ws = null; // WebSocket インスタンス
let ctx = null; // Canvas のコンテキスト
let conn = false; // WebSocket 接続状態
let canvas = null; // Canvas 要素
let secret = null; // 暗号化キー
let ticker = 0; // 定期処理のタイマー ID
let frames = 0; // 秒間フレーム数 (FPS) を計測
let bytes = 0; // 転送データ量を計測
let ticks = 0; // PING カウンタ
let title = i18n.t('DESKTOP.TITLE'); // モーダルのタイトル

// 関数コンポーネント ScreenModal を定義
function ScreenModal(props) {
	// 解像度
	const [resolution, setResolution] = useState('0x0');
	// 帯域幅 (データ転送量)
	const [bandwidth, setBandwidth] = useState(0);
	// フレームレート (FPS)
	const [fps, setFps] = useState(0);

	//Canvas の初期化
	//useCallback を使用して Canvas の初期化処理を効率化。
	// props.open (モーダルが開いている状態) を監視して、必要な初期化処理を行う。
	const canvasRef = useCallback((e) => {
		if (e && props.open && !conn && !canvas) {
			secret = hex2ua(genRandHex(32)); // 暗号化キーを生成
			canvas = e; // Canvas 要素を保存
			initCanvas(canvas); // Canvas 初期化
			construct(canvas); // WebSocket 接続を確立
		}
	}, [props]);


	// props.open が変更されたときに、websocket 接続を解除
	useEffect(() => {
		// props.open が false の場合、websocket 接続を解除
		if (!props.open) {
			canvas = null;
			// WebSocket 接続を解除
			if (ws && conn) {
				clearInterval(ticker);
				ws.close();
				conn = false;
			}
		}
	}, [props.open]);

	// Canvas の初期化
	function initCanvas() {
		if (!canvas) return;
		ctx = canvas.getContext('2d', {alpha: false});
		ctx.imageSmoothingEnabled = false; // 描画品質を調整
	}
	
	//WebSocket 接続の管理
	function construct() {
		// ctx が null でない場合、WebSocket 接続を確立
		if (ctx !== null) {
			// ws が null でない場合、既存の接続を閉じる
			if (ws !== null && conn) {
				//// 既存の接続を閉じる
				ws.close();
			}
			// serverとwebsocket接続を確立
			// server からのデスクトップ画面のストリーミングを受信するための WebSocket 接続を確立
			ws = new WebSocket(getBaseURL(true, `api/device/desktop?device=${props.device.id}&secret=${ua2hex(secret)}`));
			// バイナリ形式で通信
			ws.binaryType = 'arraybuffer';
			// WebSocket 接続が確立されたときの処理
			ws.onopen = () => {
				conn = true;
			}
			
			// WebSocket 接続がメッセージを受信したときの処理
			ws.onmessage = (e) => {
				parseBlocks(e.data, canvas, ctx);
			};

			// WebSocket 接続が閉じられたときの処理
			ws.onclose = () => {
				if (conn) {
					conn = false;
					message.warn(i18n.t('COMMON.DISCONNECTED'));
				}
			};

			// WebSocket 接続でエラーが発生したときの処理
			ws.onerror = (e) => {
				console.error(e);
				if (conn) {
					conn = false;
					message.warn(i18n.t('COMMON.DISCONNECTED'));
				} else {
					message.warn(i18n.t('COMMON.CONNECTION_FAILED'));
				}
			};

			// 定期処理のタイマーをクリア
			clearInterval(ticker);

			// 定期処理のタイマーを設定
			ticker = setInterval(() => {
				//定期的に統計情報 (帯域幅、FPS) を更新。
				setBandwidth(bytes);
				setFps(frames);
				bytes = 0;
				frames = 0;

				// PING カウンタをインクリメント
				ticks++;
				// 10秒ごとに PING メッセージを送信
				if (ticks > 10 && conn) {
					ticks = 0;
					sendData({
						act: 'DESKTOP_PING'
					});
				}
			}, 1000);
		}
	}

	//Canvas 要素 (canvas) をフルスクリーンモードに切り替える機能。
	function fullScreen() {
		//HTML5 のフルスクリーン API を使用して、Canvas 要素をフルスクリーン表示します。
		// ユーザーがフルスクリーン表示をクリックしたときに、モーダル内の Canvas を画面全体に広げます。
		canvas.requestFullscreen().catch(console.error);
	}
	function refresh() {
		// Canvas が存在し、モーダルが開いている場合
		if (canvas && props.open) {
			 // WebSocket 接続が確立されていない場合
			if (!conn) {
				// Canvas 初期化
				initCanvas(canvas);
				// WebSocket 接続の再構築
				construct(canvas);

				// WebSocket 接続が既に確立されている場合
			} else {
				 // サーバーに画面キャプチャ要求を送信
				 //別の関数 (parseBlocks) によって処理され、Canvas 上に描画されます。
				sendData({
					act: 'DESKTOP_SHOT'
				});
			}
		}

		// ユーザーがリフレッシュボタンを押すと、refresh 関数が呼び出されます。
		// WebSocket 接続が切断されている場合、新しい接続を確立。
		// 接続済みの場合、サーバーに現在のデスクトップ画面のキャプチャをリクエスト。
		// サーバーからのデータを受信し、リアルタイムで Canvas 上に描画。
	}

	//描画処理
	//リモートデスクトップ画面をリアルタイムで更新するために、受信したバイナリデータ (ab) を解析し、Canvas に描画する処理を行っています。WebSocket 経由で送られてくるデータを分解して、解像度や画像データを適切に処理するのが目的です。
	//操作コード (op) に応じて、画面の描画・更新を行う。
	function parseBlocks(ab, canvas, canvasCtx) {
		
		ab = ab.slice(5);// ヘッダー部分をスキップ
		let dv = new DataView(ab); // バイナリデータを DataView に変換
		let op = dv.getUint8(0); // 操作コード (op) を取得
		
		// JSON データの処理
		if (op === 3) {
			handleJSON(ab.slice(1));
			return;
		}

		// 解像度変更の処理
		if (op === 2) {
			// 解像度を取得
			//オフセット位置 (3 バイト目と 5 バイト目) から 2 バイトずつを読み取り、幅 (width) と高さ (height) を取得。
			let width = dv.getUint16(3, false);
			let height = dv.getUint16(5, false);
			if (width === 0 || height === 0) return;

			// 解像度を更新
			canvas.width = width;
			canvas.height = height;
			setResolution(`${width}x${height}`);
			return;
		}

		// フレーム更新の処理
		//フレーム数 (FPS 計測用) を増加。
		if (op === 0) frames++;
		//受信データ量を加算し、帯域幅の計測に利用。
		bytes += ab.byteLength;
		let offset = 1;

		// 画像ブロックの処理
		//データが複数の画像ブロックに分割されている場合、すべてのブロックを順に処理。
		while (offset < ab.byteLength) {
			//ブロックデータの取得
			// bl: ブロック全体の長さ。
			// it: 画像の種類 (例えば、画像フォーマット)。
			// dx, dy: ブロックの描画位置 (Canvas 上の座標)。
			// bw, bh: ブロックの幅と高さ。
			// il: 実際の画像データの長さ (bl - 10)。
			let bl = dv.getUint16(offset + 0, false); // body length
			let it = dv.getUint16(offset + 2, false); // image type
			let dx = dv.getUint16(offset + 4, false); // image block x
			let dy = dv.getUint16(offset + 6, false); // image block y
			let bw = dv.getUint16(offset + 8, false); // image block width
			let bh = dv.getUint16(offset + 10, false); // image block height
			let il = bl - 10; // image length
			offset += 12;

			//画像データを Canvas に描画する関数を呼び出し。
			updateImage(ab.slice(offset, offset + il), it, dx, dy, bw, bh, canvasCtx);
			offset += il;
		}

		//メモリ解放
		//処理終了後に DataView オブジェクトへの参照を解除し、メモリリークを防止。
		dv = null;
	}


	//受信した画像データ (ab) を Canvas API を用いて指定された位置やサイズに描画します。画像の形式 (it) に応じて異なる処理が行われます。
	// ab: バイナリ形式の画像データ (ArrayBuffer)。
	// it: 画像の種類を示すコード (0 または 1)。
	// dx, dy: 描画先の x, y 座標 (Canvas 上の位置)。
	// bw, bh: 画像の幅と高さ (ブロックサイズ)。
	// canvasCtx: Canvas の描画コンテキスト (2D)。
	function updateImage(ab, it, dx, dy, bw, bh, canvasCtx) {
		//画像形式に基づく処理の分岐
		switch (it) {
			//データはピクセル値そのものを表し、Canvas API の putImageData を使用して描画。
			//ピクセル値データの処理 (Case 0)
			case 0:
				// ピクセル値データの処理 (Case 0)
				canvasCtx.putImageData(new ImageData(new Uint8ClampedArray(ab), bw, bh), dx, dy, 0, 0, bw, bh);
				break;

			// 画像データのデコードと描画 (Case 1)
			//データは画像形式でエンコードされており、createImageBitmap を用いてデコードし描画。
			case 1:
				//ab をバイナリデータとしてラップし、画像データとして扱える形に変換。
				//Blob をデコードして画像オブジェクトを生成する非同期関数。
				//premultiplyAlpha: 'none':
				// アルファ値 (透明度) を無変換。
				// colorSpaceConversion: 'none':
				// 色空間変換を無効化。
				createImageBitmap(new Blob([ab]), 0, 0, bw, bh, {
					premultiplyAlpha: 'none',
					colorSpaceConversion: 'none'
				}).then((ib) => {
					//デコード済みの画像 (ib) を、Canvas 上に指定位置とサイズで描画。
					canvasCtx.drawImage(ib, 0, 0, bw, bh, dx, dy, bw, bh);
				});
				break;
		}
	}

	// JSON データの処理
	function handleJSON(ab) {
		// JSON データを復号化
		let data = decrypt(ab, secret);
		try {
			data = JSON.parse(data);
		} catch (_) {}

		// act プロパティに応じて処理を分岐
		if (data?.act === 'WARN') {
			message.warn(data.msg ? translate(data.msg) : i18n.t('COMMON.UNKNOWN_ERROR'));
			return;
		}
		if (data?.act === 'QUIT') {
			message.warn(data.msg ? translate(data.msg) : i18n.t('COMMON.UNKNOWN_ERROR'));
			conn = false;
			ws.close();
		}
	}

	// データの送信
	function sendData(data) {
		if (conn) {
			let body = encrypt(str2ua(JSON.stringify(data)), secret);
			let buffer = new Uint8Array(body.length + 8);
			buffer.set(new Uint8Array([34, 22, 19, 17, 20, 3]), 0);
			buffer.set(new Uint8Array([body.length >> 8, body.length & 0xFF]), 6);
			buffer.set(body, 8);
			ws.send(buffer);
		}
	}

	//モーダルの描画
	//モーダル内に canvas 要素を配置し、リモートデスクトップ画面を描画。
	// フルスクリーンとリフレッシュのボタンを提供。
	return (
		<DraggableModal
			draggable={true}
			maskClosable={false}
			destroyOnClose={true}
			modalTitle={`${title} ${resolution} ${formatSize(bandwidth)}/s FPS: ${fps}`}
			footer={null}
			height={480}
			width={940}
			bodyStyle={{
				padding: 0
			}}
			{...props}
		>
			<canvas
				id='painter'
				ref={canvasRef}
				style={{width: '100%', height: '100%'}}
			/>
			<Button
				style={{right:'59px'}}
				className='header-button'
				icon={<FullscreenOutlined />}
				onClick={fullScreen}
			/>
			<Button
				style={{right:'115px'}}
				className='header-button'
				icon={<ReloadOutlined />}
				onClick={refresh}
			/>
		</DraggableModal>
	);
}

export default ScreenModal;