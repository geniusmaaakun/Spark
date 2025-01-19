import React, {useEffect, useRef, useState} from "react";
import {Alert, Button, Dropdown, Menu, message, Modal, Space, Spin} from "antd";
import i18n from "../../locale/locale";
import {preventClose, waitTime} from "../../utils/utils";
import Qs from "qs";
import axios from "axios";
import {CloseOutlined, LoadingOutlined} from "@ant-design/icons";
import AceEditor from "react-ace";
import AceBuilds from "ace-builds";
import "ace-builds/src-min-noconflict/ext-language_tools";
import "ace-builds/src-min-noconflict/ext-searchbox";
import "ace-builds/src-min-noconflict/ext-modelist";

//React と Ace Editor を使用して作成された TextEditor コンポーネントです。
//このエディタは、リモートデバイス上のファイルを編集できるインターフェースを提供

// 全体の目的
// Ace Editor を使用してテキストファイルを編集。
// 編集内容を保存、検索、置換する機能を提供。
// ユーザーがフォントサイズやテーマをカスタマイズできる。
// 編集中に保存していない変更がある場合の確認をサポート。

// まとめ
// 主要機能:
// テキスト編集、保存、テーマ変更、フォントサイズ変更。
// 保存していない変更を確認する安全対策。
// 拡張性:
// Ace Editor の豊富なプラグイン (例: 自動補完、検索/置換) を活用。
// リモート連携:
// サーバーと連携してファイルを保存。

// 0: not modified, 1: modified but not saved, 2: modified and saved.

//ext-language_tools の役割
// 言語ツール (Language Tools) は、以下の機能をエディタに提供します:
// 基本的な自動補完:
// キーワードの提案や補完を実現。
// カスタム補完:
// 自前で定義した補完候補を利用可能。
// コードスニペット:
// プリセットされたコードスニペットの挿入。
const ModeList = AceBuilds.require("ace/ext/modelist");
let fileStatus = 0;
let fileChanged = false;
let editorConfig = getEditorConfig();
try {
	//node_modules フォルダ内に存在
	require('ace-builds/src-min-noconflict/theme-' + editorConfig.theme);
} catch (e) {
	require('ace-builds/src-min-noconflict/theme-idle_fingers');
	editorConfig.theme = 'Idle Fingers';
}

function TextEditor(props) {
	//ステート管理
	// 	cancelConfirm:
	// 編集内容が保存されていない場合の確認モーダル表示用。
	// fileContent:
	// 編集中のテキストデータを保持。
	// editorTheme:
	// 現在選択されているエディタのテーマ (デフォルトは idle_fingers)。
	// editorMode:
	// 編集中のファイルのモード (プログラミング言語など)。
	// loading:
	// 保存処理中に表示するローディング状態。
	const [cancelConfirm, setCancelConfirm] = useState(false); // キャンセル確認ダイアログの表示状態
	const [fileContent, setFileContent] = useState(''); // 現在編集中のファイル内容
	const [editorTheme, setEditorTheme] = useState(editorConfig.theme); // エディタテーマ
	const [editorMode, setEditorMode] = useState('text'); // エディタモード (言語設定)
	const [loading, setLoading] = useState(false);  // 保存処理中かどうか
	const [open, setOpen] = useState(props.file);  // モーダルの開閉状態
	const editorRef = useRef();  // エディタの参照

	const fontMenu = (
		<Menu onClick={onFontMenuClick}>
			<Menu.Item key='enlarge'>{i18n.t('EXPLORER.ENLARGE')}</Menu.Item>
			<Menu.Item key='shrink'>{i18n.t('EXPLORER.SHRINK')}</Menu.Item>
		</Menu>
	);
	const editorThemes = {
		'github': 'GitHub',
		'monokai': 'Monokai',
		'tomorrow': 'Tomorrow',
		'twilight': 'Twilight',
		'eclipse': 'Eclipse',
		'kuroir': 'Kuroir',
		'xcode': 'XCode',
		'idle_fingers': 'Idle Fingers',
	}
	const themeMenu = (
		<Menu onClick={onThemeMenuClick}>
			{Object.keys(editorThemes).map(key =>
				<Menu.Item disabled={editorTheme === key} key={key}>
					{editorThemes[key]}
				</Menu.Item>
			)}
		</Menu>
	);

	//初期設定とファイル読み込み
	// 	ファイルモードの設定:
	// ModeList.getModeForPath を使ってファイル拡張子に基づくモードを取得 (例: .js → javascript)。
	// 初期状態のリセット:
	// ページ離脱の確認イベントや保存状態を初期化。
	useEffect(() => {
		if (props.file) {
			let fileMode = ModeList.getModeForPath(props.file); // ファイルの拡張子からエディタモードを取得
			if (!fileMode) { // デフォルトは `text`
				fileMode = { name: 'text' };
			}
			try {
				require('ace-builds/src-min-noconflict/mode-' + fileMode.name); // 必要なモードをロード
			} catch (e) {
				require('ace-builds/src-min-noconflict/mode-text'); // ロード失敗時は `text` モード
			}
			setOpen(true);
			setFileContent(props.content); // 初期コンテンツを設定
			setEditorMode(fileMode);
		}
		fileStatus = 0; // ファイル状態をリセット (未変更)
		setCancelConfirm(false);
		window.onbeforeunload = null; // ページ離脱確認を無効化
	}, [props.file]);

	// フォントサイズ変更
	//フォントサイズを増減し、最小サイズ (14) を下回らないよう制限。
	function onFontMenuClick(e) {
		let currentFontSize = parseInt(editorRef.current.editor.getFontSize());
		currentFontSize = isNaN(currentFontSize) ? 15 : currentFontSize;
		if (e.key === 'enlarge') {
			currentFontSize++;
			editorRef.current.editor.setFontSize(currentFontSize + 1);
		} else if (e.key === 'shrink') {
			if (currentFontSize <= 14) {
				message.warn(i18n.t('EXPLORER.REACHED_MIN_FONT_SIZE'));
				return;
			}
			currentFontSize--;
			editorRef.current.editor.setFontSize(currentFontSize);
		}
		editorConfig.fontSize = currentFontSize;
		setEditorConfig(editorConfig);
	}

	//テーマ変更
	//選択されたテーマを動的にロードし、エディタに適用。
	function onThemeMenuClick(e) {
		require('ace-builds/src-min-noconflict/theme-' + e.key);
		setEditorTheme(e.key);
		editorConfig.theme = e.key;
		setEditorConfig(editorConfig);
		editorRef.current.editor.setTheme('ace/theme/' + e.key);
	}
	function onForceCancel(reload) {
		setCancelConfirm(false);
		setTimeout(() => {
			setOpen(false);
			setFileContent('');
			window.onbeforeunload = null;
			props.onCancel(reload);
		}, 150);
	}
	function onExitCancel() {
		setCancelConfirm(false);
	}

	//保存していない変更の確認
	//保存していない変更がある場合、確認モーダルを表示。
	function onCancel() {
		if (loading) return; // 保存中はキャンセル不可
		if (fileStatus === 1) { 
			setCancelConfirm(true); // 保存していない変更がある場合、確認ダイアログを表示
		} else {
			setOpen(false);
			setFileContent('');
			window.onbeforeunload = null;
			props.onCancel(fileStatus === 2); // 保存済みかどうかをコールバックで通知
		}
	}

	//編集内容の保存
	//エディタの内容をリモートサーバーに保存。
	// 保存が成功すると、fileStatus を 2 (保存済み) に更新。
	async function onConfirm(onSave) {
		if (loading) return;
		setLoading(true);
		await waitTime(300); // 少し待機してから保存処理を開始
		const params = Qs.stringify({
			device: props.device.id,
			path: props.path,
			file: props.file
		});
		axios.post(
			'/api/device/file/upload?' + params,
			editorRef.current.editor.getValue(),  // 現在のエディタ内容を送信
			{
				headers: { 'Content-Type': 'application/octet-stream' },
				timeout: 10000
			}
		).then(res => {
			let data = res.data;
			if (data.code === 0) {
				fileStatus = 2; // 保存済み状態に設定
				window.onbeforeunload = null;
				message.success(i18n.t('EXPLORER.FILE_SAVE_SUCCESSFULLY'));
				if (typeof onSave === 'function') onSave(); // 保存成功時のコールバック
			}
		}).catch(err => {
			message.error(i18n.t('EXPLORER.FILE_SAVE_FAILED') + i18n.t('COMMON.COLON') + err.message);
		}).finally(() => {
			setLoading(false);
		});
	}

	//エディタ描画 (AceEditor)
	//主要な設定:
	// mode: ファイルの種類に応じたシンタックスハイライトを設定。
	// theme: 選択されたテーマを適用。
	// onChange:
	// 内容が変更されたら未保存状態に変更 (fileStatus = 1)。
	// ページ離脱時に警告を表示。

	//キャンセル確認ダイアログ (Modal):
	// 	選択肢:
	// 保存せず閉じる。
	// 保存して閉じる。
	// キャンセル。
	return (
		<Modal
			title={props.file}
			mask={false}
			keyboard={false}
			open={open}
			maskClosable={false}
			className='editor-modal'
			closeIcon={loading ? <Spin indicator={<LoadingOutlined />} /> : <CloseOutlined />}
			onCancel={onCancel}
			footer={null}
			destroyOnClose
		>
			<Alert
				closable={false}
				message={
					<Space size={16}>
						<a onClick={onConfirm}>
							{i18n.t('EXPLORER.SAVE')}
						</a>
						<a onClick={()=>editorRef.current.editor.execCommand('find')}>
							{i18n.t('EXPLORER.SEARCH')}
						</a>
						<a onClick={()=>editorRef.current.editor.execCommand('replace')}>
							{i18n.t('EXPLORER.REPLACE')}
						</a>
						<Dropdown overlay={fontMenu}>
							<a>{i18n.t('EXPLORER.FONT')}</a>
						</Dropdown>
						<Dropdown overlay={themeMenu}>
							<a>{i18n.t('EXPLORER.THEME')}</a>
						</Dropdown>
					</Space>
				}
				style={{marginBottom: '12px'}}
			/>
			<AceEditor
				ref={editorRef}
				mode={editorMode.name}
				theme={editorTheme}
				name='text-editor'
				width='100%'
				height='100%'
				commands={[{
					name: 'save',
					bindKey: {win: 'Ctrl-S', mac: 'Command-S'},
					exec: onConfirm
				}, {
					name: 'find',
					bindKey: {win: 'Ctrl-F', mac: 'Command-F'},
					exec: 'find'
				}, {
					name: 'replace',
					bindKey: {win: 'Ctrl-H', mac: 'Command-H'},
					exec: 'replace'
				}]}
				value={fileContent}
				onChange={val => {
					if (!open) return;
					if (val.length === fileContent.length) {
						if (val === fileContent) return;
					}
					window.onbeforeunload = preventClose;
					setFileContent(val);
					fileStatus = 1;
				}}
				debounceChangePeriod={100}
				fontSize={editorConfig.fontSize}
				editorProps={{ $blockScrolling: true }}
				setOptions={{
					enableBasicAutocompletion: true
				}}
			/>
			<Modal
				closable={true}
				open={cancelConfirm}
				onCancel={onExitCancel}
				footer={[
					<Button
						key='cancel'
						onClick={onExitCancel}
					>
						{i18n.t('EXPLORER.CANCEL')}
					</Button>,
					<Button
						type='danger'
						key='doNotSave'
						onClick={onForceCancel.bind(null, false)}
					>
						{i18n.t('EXPLORER.FILE_DO_NOT_SAVE')}
					</Button>,
					<Button
						type='primary'
						key='save'
						onClick={onConfirm.bind(null, onForceCancel.bind(null, true))}
					>
						{i18n.t('EXPLORER.SAVE')}
					</Button>
				]}
			>
				{i18n.t('EXPLORER.NOT_SAVED_CONFIRM')}
			</Modal>
		</Modal>
	);
}
function getEditorConfig() {
	let config = localStorage.getItem('editorConfig');
	if (config) {
		try {
			config = JSON.parse(config);
		} catch (e) {
			config = null;
		}
	}
	if (!config) {
		config = {
			fontSize: 15,
			theme: 'idle_fingers',
		};
	}
	return config;
}
function setEditorConfig(config) {
	localStorage.setItem('editorConfig', JSON.stringify(config));
}

export default TextEditor;