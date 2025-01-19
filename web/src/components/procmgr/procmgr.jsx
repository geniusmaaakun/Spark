import React, {useEffect, useMemo, useRef, useState} from 'react';
import {Button, message, Popconfirm} from "antd";
import ProTable from '@ant-design/pro-table';
import {request, waitTime} from "../../utils/utils";
import i18n from "../../locale/locale";
import {VList} from "virtuallist-antd";
import DraggableModal from "../modal";
import {ReloadOutlined} from "@ant-design/icons";

//React を使用してプロセスマネージャー（ProcessMgr）を実装したものです。
//このコンポーネントは、リモートデバイスのプロセスリストを表示し、特定のプロセスを終了する機能を提供

// 全体の目的
// プロセスリストの表示:
// プロセス名や PID（プロセスID）を表形式で表示。
// プロセスの終了:
// 選択したプロセスを終了（Kill）する操作を提供。

// まとめ
// 主要機能:
// プロセスリストの表示。
// プロセスの終了。
// UI 特徴:
// ProTable を使用した柔軟なテーブル表示。
// 仮想スクロールでパフォーマンスを向上。
// モーダル内でプロセスマネージャを動的に操作。
// 操作性:
// 確認ダイアログで誤操作を防止。
// テーブルのリロードボタンを提供。


function ProcessMgr(props) {
	//コンポーネントのステート管理
	//loading:
	// プロセスリストのデータ取得中にローディング状態を表示するためのフラグ。
	const [loading, setLoading] = useState(false); // ローディング状態を管理
	
	//表示する列の定義
	// 	列の役割:
	// Name: プロセスの名前を表示。
	// Pid: プロセスIDを表示。
	// Option: 操作ボタンを表示する列。
	const columns = [
		{
			key: 'Name',
			title: i18n.t('PROCMGR.PROCESS'),  // プロセス名
			dataIndex: 'name',
			ellipsis: true,
			width: 120
		},
		{
			key: 'Pid',
			title: 'Pid', // プロセスID
			dataIndex: 'pid',
			ellipsis: true,
			width: 40
		},
		{
			key: 'Option',
			width: 40,
			title: '', // 操作メニュー
			dataIndex: 'name',
			valueType: 'option',
			ellipsis: true,
			render: (_, file) => renderOperation(file) // 操作ボタン
		},
	];

	const options = {
		show: true,
		reload: false,
		density: false,
		setting: false,
	};
	const tableRef = useRef();
	const virtualTable = useMemo(() => {
		return VList({
			height: 300
		})
	}, []);

	useEffect(() => {
		if (props.open) {
			setLoading(false);
		}
	}, [props.device, props.open]);

	//操作ボタンの表示
	// 確認メッセージ
	// 確認時にプロセス終了
	// 操作ボタン
	// 	役割:
	// 「Kill」ボタンを表示し、クリック時に確認ダイアログを表示。
	// ダイアログで「OK」を押すと、選択したプロセスを終了。
	function renderOperation(proc) {
		return [
			<Popconfirm
				key='kill'
				title={i18n.t('PROCMGR.KILL_PROCESS_CONFIRM')} 
				onConfirm={killProcess.bind(null, proc.pid)} 
			>
				<a>{i18n.t('PROCMGR.KILL_PROCESS')}</a>  
			</Popconfirm>
		];
	}

	//プロセスの終了
	// 	役割:
	// API にリクエストを送信して、指定されたプロセス（PID）を終了。
	// 成功後、プロセスリストを再読み込み。
	function killProcess(pid) {
		request(`/api/device/process/kill`, {pid: pid, device: props.device.id}).then(res => {
			let data = res.data;
			if (data.code === 0) {
				message.success(i18n.t('PROCMGR.KILL_PROCESS_SUCCESSFULLY')); // 成功メッセージを表示
				tableRef.current.reload(); // テーブルをリロードしてリストを更新
			}
		});
	}

	//プロセスリストの取得
	// 	役割:
	// API リクエストを送信してプロセスリストを取得。
	// PID 順にソートしてデータを整形。
	// 取得成功時: 整形されたプロセスリストを返す。
	// 取得失敗時: 空のデータを返す。
	async function getData(form) {
		await waitTime(300);  // 読み込み遅延のシミュレーション
		let res = await request('/api/device/process/list', {device: props.device.id});
		setLoading(false);
		let data = res.data;
		if (data.code === 0) {
			// PID でソート
			data.data.processes = data.data.processes.sort((first, second) => (second.pid - first.pid));
			return ({
				data: data.data.processes,
				success: true,
				total: data.data.processes.length
			});
		}
		return ({data: [], success: false, total: 0});
	}

	//モーダル表示
	//DraggableModal:
	// プロセスマネージャをモーダル内に表示。
	// 背景をクリックして閉じないように設定。
	// 	<DraggableModal
	//     draggable={true} // モーダルをドラッグ可能に
	//     maskClosable={false} // 背景クリックで閉じない
	//     destroyOnClose={true} // 閉じた際に内容を破棄
	//     modalTitle={i18n.t('PROCMGR.TITLE')} // タイトル
	//     footer={null}
	//     width={500} // モーダルの幅
	//     bodyStyle={{
	//         padding: 0
	//     }}
	//     {...props}
	// />

	//テーブル設定と ProTable
	//ポイント:
	// ProTable を使用してプロセスリストを表示。
	// actionRef を使用してテーブルのリロード操作を外部から可能に。
	// 仮想スクロール（virtualTable）を使用してパフォーマンスを最適化。
	// 	<ProTable
	//     rowKey='pid' // 行の一意識別子として PID を使用
	//     tableStyle={{
	//         paddingTop: '20px',
	//         minHeight: '355px',
	//         maxHeight: '355px'
	//     }}
	//     scroll={{scrollToFirstRowOnChange: true, y: 300}} // スクロールを設定
	//     search={false} // 検索バーを非表示
	//     size='small'
	//     loading={loading} // ローディング状態を反映
	//     onLoadingChange={setLoading}
	//     options={options} // テーブルオプションを設定
	//     columns={columns} // 列の定義を適用
	//     request={getData} // データ取得ロジック
	//     pagination={false} // ページネーションを無効化
	//     actionRef={tableRef} // テーブルの再読み込みを操作可能に
	//     components={virtualTable} // 仮想スクロールを適用
	// />

	// 	再読み込みボタン
	// コピーする
	// 編集する
	// <Button
	//     style={{right: '59px'}}
	//     className='header-button'
	//     icon={<ReloadOutlined />}
	//     onClick={() => {
	//         tableRef.current.reload(); // テーブルをリロード
	//     }}
	// />
	// 役割:
	// テーブルの内容を手動で再読み込みするボタン。
	return (
		<DraggableModal
			draggable={true}
			maskClosable={false}
			destroyOnClose={true}
			modalTitle={i18n.t('PROCMGR.TITLE')}
			footer={null}
			width={500}
			bodyStyle={{
				padding: 0
			}}
			{...props}
		>
			<ProTable
				rowKey='pid'
				tableStyle={{
					paddingTop: '20px',
					minHeight: '355px',
					maxHeight: '355px'
				}}
				scroll={{scrollToFirstRowOnChange: true, y: 300}}
				search={false}
				size='small'
				loading={loading}
				onLoadingChange={setLoading}
				options={options}
				columns={columns}
				request={getData}
				pagination={false}
				actionRef={tableRef}
				components={virtualTable}
			>
			</ProTable>
			<Button
				style={{right:'59px'}}
				className='header-button'
				icon={<ReloadOutlined />}
				onClick={() => {
					tableRef.current.reload();
				}}
			/>
		</DraggableModal>
	)
}

export default ProcessMgr;