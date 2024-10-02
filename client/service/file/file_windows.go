//go:build windows
// +build windows

package file

import "github.com/shirou/gopsutil/v3/disk"

/*
Windows環境で特定のパスにあるファイルのリストを取得するための実装です。特に、ルートディレクトリ（\ や /）を対象とした場合に、システムに存在するすべてのボリュームのマウントポイント（ドライブ）を取得し、それをリストとして返します。

ルートディレクトリの場合: Windowsのシステムに存在するすべてのボリュームのマウントポイントを取得し、そのリストを返します。
ルートディレクトリでない場合: 指定された path にあるファイルのリストを返します。
*/

/*
func ListFiles(path string) ([]File, error)

この関数は path というファイルパスを引数に取り、そのパスにあるファイルのリストを返します。
戻り値は、File のスライスと error です。File 構造体は各ファイルやディレクトリの情報を持っていると想定されます。
result := make([]File, 0)

result という空の File のスライスを作成します。ここに結果を格納します。
ルートディレクトリのチェック

if len(path) == 0 || path == ` || path == /`` では、pathがルートディレクトリかどうかをチェックしています。Windowsのルートディレクトリは` や / で表現されます。
もし path がルートディレクトリなら、システムのすべてのボリュームを取得します。
partitions, err := disk.Partitions(true)

disk.Partitions(true) を使って、システム内のすべてのパーティション（ボリューム）のマウントポイントを取得します。err にはエラーがあればその情報が格納されます。
for i := 0; i < len(partitions); i++ { ... }

取得したパーティションをループして、各ボリュームの情報を処理します。
disk.Usage(partitions[i].Mountpoint)

各パーティションのマウントポイントに対してディスク使用量の情報を取得します。例えば、そのボリュームの総容量 (stat.Total) を取得し、size に格納します。
もし disk.Usage でエラーが発生したり、情報が取得できなかった場合は size = 0 とします。
result = append(result, File{Name: partitions[i].Mountpoint, Type: 2, Size: size})

result スライスに各パーティションの情報を File 構造体として追加します。Name にはマウントポイント（例: C:\ など）、Type にはディレクトリを表す2、Size にはパーティションの総容量をセットします。
return result, nil

パーティション情報を全て収集したら、result スライスを返します。エラーがなければ nil を返します。
return listFiles(path)

ルートディレクトリでない場合、通常のファイルリスト取得処理である listFiles(path) を呼び出して、そのパスのファイル一覧を取得します。

*/
// ListFiles will only be called when path is root and
// current system is Windows.
// It will return mount points of all volumes.
func ListFiles(path string) ([]File, error) {
	result := make([]File, 0)
	if len(path) == 0 || path == `\` || path == `/` {
		partitions, err := disk.Partitions(true)
		if err != nil {
			return nil, err
		}
		for i := 0; i < len(partitions); i++ {
			size := uint64(0)
			stat, err := disk.Usage(partitions[i].Mountpoint)
			if err != nil || stat == nil {
				size = 0
			} else {
				size = stat.Total
			}
			result = append(result, File{Name: partitions[i].Mountpoint, Type: 2, Size: size})
		}
		return result, nil
	}
	return listFiles(path)
}
