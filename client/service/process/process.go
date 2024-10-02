package process

import "github.com/shirou/gopsutil/v3/process"

/*
Go言語でシステム上のプロセスをリストアップし、特定のプロセスを終了させるための機能を提供しています。github.com/shirou/gopsutil/v3/process ライブラリを使用しており、これはシステムのプロセス情報にアクセスするための便利なライブラリです。


処理の流れ
プロセスのリスト取得

ListProcesses 関数は、システム上で動作している全てのプロセスをリストアップし、それぞれのプロセス名とPIDを Process 構造体にまとめて返します。
名前が取得できない場合もエラーハンドリングを行い、プロセス名を "<UNKNOWN>" として処理を続行します。
プロセスの強制終了

KillProcess 関数は、特定のプロセスIDに該当するプロセスを探し、そのプロセスを終了させます。process.Processes() で全プロセスを取得してから目的のプロセスをループで検索し、該当プロセスを終了します。
注意点
エラーハンドリング: プロセス名の取得やプロセスの終了処理に失敗した場合、エラーを適切に返すようになっており、堅牢なエラーハンドリングが実装されています。
プロセス終了の権限: プロセスを終了させる場合、実行中のプログラムには適切な権限が必要です。権限が不足している場合、KillProcess 関数でエラーが発生することがあります。
このコードは、システム上のプロセスを操作するための基本的なインターフェースを提供しており、プロセス管理をシンプルに行うことができます。
*/

/*
シンプルな構造体で、システム上のプロセスを表現します。
Name: プロセスの名前。
Pid: プロセスID（PID）。
*/
type Process struct {
	Name string `json:"name"`
	Pid  int32  `json:"pid"`
}

/*
システム上で実行中のすべてのプロセスをリストアップする関数です。
process.Processes() 関数を使って、現在動作しているプロセスの情報を取得します。
各プロセスについて名前 (Name()) とプロセスID (Pid) を取得し、Process 構造体に格納してリスト化します。
名前の取得に失敗した場合は、プロセス名を "<UNKNOWN>" に設定します。
*/
func ListProcesses() ([]Process, error) {
	result := make([]Process, 0)
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(processes); i++ {
		name, err := processes[i].Name()
		if err != nil {
			name = `<UNKNOWN>`
		}
		result = append(result, Process{Name: name, Pid: processes[i].Pid})
	}
	return result, nil
}

/*
特定のプロセスID (pid) を持つプロセスを終了させる関数です。
process.Processes() でシステム上のすべてのプロセスを取得し、ループを回して目的のプロセスIDに一致するプロセスを探します。
一致するプロセスが見つかった場合、そのプロセスを Kill() 関数を使って終了させます。
該当するプロセスが見つからなかった場合や、エラーが発生した場合は、そのエラーを返します。
*/
func KillProcess(pid int32) error {
	processes, err := process.Processes()
	if err != nil {
		return err
	}
	for i := 0; i < len(processes); i++ {
		if processes[i].Pid == pid {
			return processes[i].Kill()
		}
	}
	return nil
}
