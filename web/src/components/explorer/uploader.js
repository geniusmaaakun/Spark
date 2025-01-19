import React, {useEffect, useState} from "react";
import Qs from "qs";
import {formatSize, preventClose} from "../../utils/utils";
import axios from "axios";
import {message, Modal, Progress, Typography} from "antd";
import i18n from "../../locale/locale";
import DraggableModal from "../modal";

//FileUploader コンポーネントで、リモートデバイスにファイルをアップロードするためのインターフェースを提供
// 全体の目的
// ファイルアップロードを行うためのモーダルウィンドウを表示。
// アップロードの進捗状況をリアルタイムで表示。
// ユーザーがアップロードをキャンセルできる仕組みを提供。
// アップロード成功、失敗、キャンセル時に適切な通知を表示。

// まとめ
// 機能概要:
// ファイルアップロード、進捗表示、キャンセル機能を提供。
// 状態管理:
// アップロードの進行状況や結果に応じた状態遷移。
// UI 表示:
// Ant Design を活用した進捗バーと通知。
// ユーザー体験向上:
// 中断可能なアップロード、失敗/成功時のフィードバックを実装。
// このコンポーネントにより、ユーザーは簡単にリモートデバイスにファイルをアップロードできます。

let abortController = null;
function FileUploader(props) {
	//ステートの管理
	const [open, setOpen] = useState(!!props.file); // モーダルの開閉状態
	const [percent, setPercent] = useState(0); // アップロード進捗 (%)
	const [status, setStatus] = useState(0); // アップロード状態
	// 0: ready, 1: uploading, 2: success, 3: fail, 4: cancel

	//ライフサイクル管理 (useEffect)
	//ファイルが選択されるたびに (props.file の変更時) 以下を実行:
	// 状態を初期化 (status = 0, percent = 0)。
	// モーダルを開く。
	useEffect(() => {
		setStatus(0);
		if (props.file) {
			setOpen(true); // モーダルを開く
			setPercent(0);  // 進捗をリセット
		}
	}, [props.file]);

	//アップロード開始 (onConfirm)
	function onConfirm() {
		if (status !== 0) {
			onCancel();
			return;
		}
		const params = Qs.stringify({
			device: props.device.id,
			path: props.path,
			file: props.file.name
		});
		let uploadStatus = 1;
		setStatus(1); // アップロード中に設定
		window.onbeforeunload = preventClose; // ページ離脱防止
		abortController = new AbortController(); // アップロード中断用のコントローラ
		axios.post(
			'/api/device/file/upload?' + params,
			props.file,  // アップロードするファイル
			{
				headers: {
					'Content-Type': 'application/octet-stream'
				},
				timeout: 0,
				onUploadProgress: (progressEvent) => {
					let percentCompleted = Math.round((progressEvent.loaded * 100) / progressEvent.total);
					setPercent(percentCompleted); // 進捗状況を更新
				},
				signal: abortController.signal // 中断用シグナル
			}
		).then(res => {
			let data = res.data;
			if (data.code === 0) {
				uploadStatus = 2; // 成功
				setStatus(2);
				message.success(i18n.t('EXPLORER.UPLOAD_SUCCESS'));
			} else {
				uploadStatus = 3; // 失敗
				setStatus(3);
			}
		}).catch(err => {
			if (axios.isCancel(err)) {
				uploadStatus = 4; // キャンセル
				setStatus(4);
				message.error(i18n.t('EXPLORER.UPLOAD_ABORTED'));
			} else {
				uploadStatus = 3; // 失敗
				setStatus(3);
				message.error(i18n.t('EXPLORER.UPLOAD_FAILED') + i18n.t('COMMON.COLON') + err.message);
			}
		}).finally(() => {
			abortController = null;
			window.onbeforeunload = null; // ページ離脱防止を解除
			setTimeout(() => {
				setOpen(false); // モーダルを閉じる
				if (uploadStatus === 2) {
					props.onSuccess(); // アップロード成功時のコールバック
				} else {
					props.onCancel(); // その他の状態時のコールバック
				}
			}, 1500);
		});

	// 	主要なポイント:
	// 進捗更新: onUploadProgress で進捗イベントを処理し、状態を更新。
	// 中断機能: AbortController を用いてアップロードをキャンセル可能。
	// 結果処理:
	// 成功時 (status = 2): 成功通知を表示し、props.onSuccess を呼び出す。
	// 失敗時 (status = 3): エラー通知を表示。
	// キャンセル時 (status = 4): キャンセル通知を表示。
	}

	//アップロードキャンセル (onCancel)
	function onCancel() {
		if (status === 0) {
			setOpen(false);  // モーダルを閉じる 
			setTimeout(props.onCancel, 300); // キャンセルコールバック
			return;
		}
		if (status === 1) {
			Modal.confirm({
				autoFocusButton: 'cancel',
				content: i18n.t('EXPLORER.UPLOAD_CANCEL_CONFIRM'),
				onOk: () => {
					abortController.abort(); // アップロードを中断
				},
				okButtonProps: {
					danger: true,
				},
			});
			return;
		}
		setTimeout(() => {
			setOpen(false); // モーダルを閉じる
			setTimeout(props.onCancel, 300);
		}, 1500);

	// 		進行中のアップロード:
	// 確認ダイアログを表示し、キャンセル操作をサポート。
	}

	//現在のアップロード状態に応じた説明テキストを返す。
	function getDescription() {
		switch (status) {
			case 1:
				return percent + '%';
			case 2:
				return i18n.t('EXPLORER.UPLOAD_SUCCESS');
			case 3:
				return i18n.t('EXPLORER.UPLOAD_FAILED');
			case 4:
				return i18n.t('EXPLORER.UPLOAD_ABORTED');
			default:
				return i18n.t('EXPLORER.UPLOAD');
		}
	}

	//描画処理 (return)
	// DraggableModal:
	// モーダル内に進捗バー (Progress) とアップロード中の状態 (getDescription()) を表示。
	// 動作中 (status = 1) のとき、モーダルを閉じたりキー操作でキャンセルできないように設定。
	return (
		<DraggableModal
			centered
			draggable
			open={open}
			closable={false}
			keyboard={false}
			maskClosable={false}
			destroyOnClose={true}
			confirmLoading={status === 1}
			okText={i18n.t(status === 1 ? 'EXPLORER.UPLOADING' : 'EXPLORER.UPLOAD')}
			modalTitle={i18n.t(status === 1 ? 'EXPLORER.UPLOADING' : 'EXPLORER.UPLOAD')}
			okButtonProps={{disabled: status !== 0}}
			cancelButtonProps={{disabled: status > 1}}
			onCancel={onCancel}
			onOk={onConfirm}
			width={550}
		>
			<div>
                <span
					style={{
						whiteSpace: 'nowrap',
						fontSize: '20px',
						marginRight: '10px',
					}}
				>
                    {getDescription()}
                </span>
				<Typography.Text
					ellipsis={{rows: 1}}
					style={{maxWidth: 'calc(100% - 140px)'}}
				>
					{props.file.name}
				</Typography.Text>
				<span
					style={{whiteSpace: 'nowrap'}}
				>
					{'（'+formatSize(props.file.size)+'）'}
				</span>
			</div>
			<Progress
				strokeLinecap='butt'
				percent={percent}
				showInfo={false}
			/>
		</DraggableModal>
	)
}

export default FileUploader;