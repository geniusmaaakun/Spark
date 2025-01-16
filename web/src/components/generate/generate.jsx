import React from 'react';
import {ModalForm, ProFormCascader, ProFormDigit, ProFormGroup, ProFormText} from '@ant-design/pro-form';
import {post, request} from "../../utils/utils";
//prebuilt:
// サーバーにリクエストする際の OS とアーキテクチャの選択肢を提供する JSON データ。
import prebuilt from '../../config/prebuilt.json';
import i18n from "../../locale/locale";

//@ant-design/pro-form を利用してモーダルフォームを作成し、サーバーにリクエストを送信してデータを生成するコンポーネント Generate を実装
// モーダルウィンドウ内にフォームを表示し、ユーザーから以下の情報を入力させます:
// ホスト名 (host)
// ポート番号 (port)
// パス (path)
// OS とアーキテクチャの選択 (ArchOS)
// 入力内容をサーバーに送信し、処理を実行する。

// URL ごとの初期値例
// URL	host	port	path	補足
// https://example.com/api/client	example.com	443	/api/client	HTTPS のデフォルトポート
// http://example.com:8080/path	example.com	8080	/path	明示的なポートを使用
// http://localhost:3000/	localhost	3000	/	開発環境のローカルサーバ
// https://sub.example.com/resource	sub.example.com	443	/resource	サブドメインの例
// まとめ
// host: URL から取得したホスト名や IP アドレスを使用。
// port: 明示的に指定されている場合はその値を使用。指定がない場合はプロトコルに基づくデフォルト値。
// path: URL のパス部分をそのまま使用。
// これらの初期値により、現在のブラウザの状態に基づいて、適切なサーバー接続情報が設定されます。

function Generate(props) {
	// 初期値の取得
	const initValues = getInitValues();

	//フォーム送信時の処理
	// 	役割:
	// フォーム入力データを処理し、サーバーにリクエストを送信。
	// 処理の流れ:
	// ArchOS のデータを os と arch に分割。
	// HTTPS 使用時は secure を true に設定。
	// check エンドポイント にリクエストを送信し、データが有効かを確認。
	// データが有効な場合、generate エンドポイント にリクエストを送信して生成処理を実行。
	async function onFinish(form) {
		if (form?.ArchOS?.length === 2) {
			form.os = form.ArchOS[0];
			form.arch = form.ArchOS[1];
			delete form.ArchOS; // ArchOS は os と arch に分割して削除
		}
		form.secure = location.protocol === 'https:' ? 'true' : 'false'; // HTTPS かどうか
		let basePath = location.origin + location.pathname + 'api/client/';
		request(basePath + 'check', form).then(res => {
			if (res.data.code === 0) {
				post(basePath += 'generate', form); // サーバーに生成リクエストを送信
			}
		}).catch();
	}

	// 初期値の取得
	//フォームに初期値を設定。
	// 現在の URL 情報 (ホスト名、ポート、パス) を取得。
	// デフォルトのポート番号を HTTP/HTTPS に応じて決定。
	function getInitValues() {
		let initValues = {
			host: location.hostname, // 現在のホスト名
			port: location.port, // 現在のポート番号
			path: location.pathname, // 現在のパス
			ArchOS: ['windows', 'amd64'] // デフォルトの OS とアーキテクチャ
		};
		if (String(location.port).length === 0) {
			initValues.port = location.protocol === 'https:' ? 443 : 80; // デフォルトのポート設定
		}
		return initValues;
	}

	// 	入力フィールド
	// ProFormText:
	// テキスト入力フィールド (ホスト名、パス)。
	// ProFormDigit:
	// 数値入力フィールド (ポート番号)。
	// 範囲は 1~65535。
	// ProFormCascader:
	// 階層的なデータ選択フィールド (OS とアーキテクチャ)。
	// prebuilt を使用して選択肢を動的に提供。
	return (
		<ModalForm
			modalProps={{
				destroyOnClose: true, // モーダルを閉じたら状態を破棄
				maskClosable: false, // 背景をクリックしても閉じない
			}}
			initialValues={initValues} // 初期値の設定
			onFinish={onFinish} // フォーム送信時の処理
			submitter={{
				render: (_, elems) => elems.pop()
			}}
			{...props}
		>
			<ProFormGroup>
				<ProFormText
					width="md"
					name="host"
					label={i18n.t('GENERATOR.HOST')}
					rules={[{
						required: true
					}]}
				/>
				<ProFormDigit
					width="md"
					name="port"
					label={i18n.t('GENERATOR.PORT')}
					min={1}
					max={65535}
					rules={[{
						required: true
					}]}
				/>
			</ProFormGroup>
			<ProFormGroup>
				<ProFormText
					width="md"
					name="path"
					label={i18n.t('GENERATOR.PATH')}
					rules={[{
						required: true
					}]}
				/>
				<ProFormCascader
					width="md"
					name="ArchOS"
					label={i18n.t('GENERATOR.OS_ARCH')}
					request={() => prebuilt} // OS/アーキテクチャの選択肢
					rules={[{
						required: true
					}]}
				/>
			</ProFormGroup>
		</ModalForm>
	)
}

export default Generate;