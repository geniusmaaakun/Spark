import React from 'react';
import {ModalForm, ProFormText} from '@ant-design/pro-form';
import {request} from "../../utils/utils";
import i18n from "../../locale/locale";
import {message} from "antd";

//リモートデバイス上でコマンドを実行するためのフォームをモーダルウィンドウとして提供

// 全体の目的
// ユーザーがリモートデバイスに対してコマンドを送信して実行できるインターフェースを提供。
// コマンドと引数を入力するフォームを表示。
// コマンド実行後、成功メッセージを表示。

// ユーザーインターフェースの動作
// ユーザーがモーダルを開き、以下の操作を実行:
// コマンド入力:
// cmd フィールドにコマンドを入力。
// 任意で args フィールドに引数を入力。
// 送信ボタンをクリック:
// サーバーにコマンドを送信。
// 成功メッセージの表示:
// コマンドが正常に実行されると、成功メッセージが表示される。
// まとめ
// 主要機能:
// モーダルフォームを使用して、リモートデバイスに対してコマンドを実行。
// コマンドと引数を入力可能。
// UI 機能:
// ProFormText を使用した入力フォーム。
// Ant Design を使用したモーダルとメッセージ表示。
// サーバー連携:
// コマンドを /api/device/exec API に送信してリモートで実行。

function Execute(props) {

	//コマンドの実行
	//処理フロー:
	// フォームデータに device.id を追加:
	// リモートデバイスを識別するため。
	// サーバーにリクエストを送信:
	// API エンドポイント: /api/device/exec
	// リクエストボディ: コマンド (cmd) と引数 (args)、デバイスID。
	// 実行成功時:
	// 成功メッセージ (EXECUTE.EXECUTION_SUCCESS) を表示。
	async function onFinish(form) {
		form.device = props.device.id; // デバイスIDをリクエストに追加
		let basePath = location.origin + location.pathname + 'api/device/'; // ベースURLを生成
		request(basePath + 'exec', form).then(res => {
			if (res.data.code === 0) {
				message.success(i18n.t('EXECUTE.EXECUTION_SUCCESS')); // 成功メッセージを表示
			}
		});
	}

	//モーダルフォームの構造
	//<ModalForm
	//     modalProps={{
	//         destroyOnClose: true, // モーダルを閉じたときに内部状態を破棄
	//         maskClosable: false,  // モーダル外をクリックしても閉じない
	//     }}
	//     title={i18n.t('EXECUTE.TITLE')} // モーダルのタイトル
	//     width={380} // モーダルの幅
	//     onFinish={onFinish} // フォーム送信時の処理
	//     onVisibleChange={open => {
	//         if (!open) props.onCancel(); // モーダルが閉じられたときの処理
	//     }}
	//     submitter={{
	//         render: (_, elems) => elems.pop() // デフォルトの送信ボタンを使用
	//     }}
	//     {...props}
	// />
	// ModalForm:
	// モーダルウィンドウ内にフォームを表示するためのコンポーネント。
	// onFinish: フォーム送信時の処理。
	// onVisibleChange:
	// モーダルの表示状態が変化したときの処理。
	// モーダルが閉じた際に親コンポーネントに通知 (props.onCancel() を呼び出す)。

	//フォームフィールドの定義
	//<ProFormText
	//     width="md"
	//     name="cmd" // 入力フィールド名
	//     label={i18n.t('EXECUTE.CMD_PLACEHOLDER')} // フィールドのラベル
	//     rules={[{
	//         required: true // 必須フィールド
	//     }]}
	// />
	// <ProFormText
	//     width="md"
	//     name="args" // 入力フィールド名
	//     label={i18n.t('EXECUTE.ARGS_PLACEHOLDER')} // フィールドのラベル
	// />
	// cmd:
	// 実行するコマンドを入力するフィールド。
	// 必須項目 (required: true)。
	// args:
	// コマンドの引数を入力するフィールド。
	// 必須ではない。

	return (
		<ModalForm
			modalProps={{
				destroyOnClose: true,
				maskClosable: false,
			}}
			title={i18n.t('EXECUTE.TITLE')}
			width={380}
			onFinish={onFinish}
			onVisibleChange={open => {
				if (!open) props.onCancel();
			}}
			submitter={{
				render: (_, elems) => elems.pop()
			}}
			{...props}
		>
			<ProFormText
				width="md"
				name="cmd"
				label={i18n.t('EXECUTE.CMD_PLACEHOLDER')}
				rules={[{
					required: true
				}]}
			/>
			<ProFormText
				width="md"
				name="args"
				label={i18n.t('EXECUTE.ARGS_PLACEHOLDER')}
			/>
		</ModalForm>
	)
}

export default Execute;