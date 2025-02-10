import React from 'react';
import ReactDOM from 'react-dom';
import {HashRouter as Router, Route, Routes} from 'react-router-dom';
import Wrapper from './components/wrapper';
import Err from './pages/404';
import axios from 'axios';
import {message} from 'antd';
import i18n from "./locale/locale";

import './global.css';
import 'antd/dist/antd.css';
import Overview from "./pages/overview";
import {translate} from "./utils/utils";

//React + React Router + Axios + Ant Design を使用して、Web アプリケーションのエントリーポイント (index.js もしくは main.js 相当) を定義✅ React + React Router を使用してルーティングを管理
// ✅ Axios のインターセプターを設定 し、API リクエストのエラーハンドリング
// ✅ Ant Design の UI コンポーネント (message) を利用
// ✅ CSS を読み込み (global.css, antd.css)
// ✅ Wrapper コンポーネントでアプリ全体をラップ
// ✅ Overview (ダッシュボード) をトップページとして表示


//axios.defaults.baseURL = '.';
// API のベース URL を設定（. = 現在のホスト）。
axios.defaults.baseURL = '.';

// レスポンス時 (response.use)
// data.code の値を確認:
// 0 → 成功（デフォルトの timeout を 5000ms に設定）
// != 0 → message.warn() で警告
// エラー時 (response.use の第 2 引数)
// ECONNABORTED（タイムアウト発生時）
// → message.error(i18n.t('COMMON.REQUEST_TIMEOUT'))
// サーバーエラー (data.code !== 0)
// message.warn(translate(data.msg));
// それ以外のエラーは Promise.reject(err); で例外を投げる。
axios.interceptors.response.use(async res => {
	let data = res.data;
	if (data.hasOwnProperty('code')) {
		if (data.code !== 0){
			message.warn(translate(data.msg));
		} else {
			// The first request will ask user to provide user/pass.
			// If set timeout at the beginning, then timeout warning
			// might be triggered before authentication finished.
			axios.defaults.timeout = 5000;
		}
	}
	return Promise.resolve(res);
}, err => {
	// console.error(err);
	if (err.code === 'ECONNABORTED') {
		message.error(i18n.t('COMMON.REQUEST_TIMEOUT'));
		return Promise.reject(err);
	}
	let res = err.response;
	let data = res?.data ?? {};
	if (data.hasOwnProperty('code') && data.hasOwnProperty('msg')) {
		if (data.code !== 0){
			message.warn(translate(data.msg));
			return Promise.resolve(res);
		}
	}
	return Promise.reject(err);
});

//React アプリケーションのルーティング 
//HashRouter を使ったルーティング
// ルートパス	コンポーネント
// /	Wrapper 内に Overview を表示 (ダッシュボード)
// * (その他すべて)	Err (404 ページ)
// 📌 Wrapper コンポーネント
// Wrapper は、アプリ全体をラップするレイアウトコンポーネント。
// Overview（メインダッシュボード）を内包している。
ReactDOM.render(
	<Router>
		<Routes>
			<Route path="/" element={<Wrapper><Overview/></Wrapper>}/>
			<Route path="*" element={<Err/>}/>
		</Routes>
	</Router>,
	document.getElementById('root')
);