package melody

import (
	"crypto/rand"
	"fmt"
)

//UUID（Universally Unique Identifier）を生成するための簡易的な関数です。UUIDは、128ビット（16バイト）のデータから構成される識別子で、通常はハイフンで区切られた5つの部分に分けて表現されます。関数はUUIDの形式に従ってランダムなデータを生成し、フォーマットしています。
func generateUUID() string {
	//16バイトのスライス buf を作成します。UUIDは128ビット、つまり16バイトのサイズであるため、ここで16バイトのバッファを用意しています。
	buf := make([]byte, 16)
	// rand.Reader は、暗号学的に安全なランダムな値を生成するためのリーダーです。bufに16バイトのランダムなデータを読み込みます。
	// このランダムデータはUUIDを生成するために使われます。暗号学的な乱数生成を使うことで、UUIDが非常にユニークになることが保証されます。
	rand.Reader.Read(buf)
	//出力形式は、xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxxのようなUUID標準形式（8-4-4-4-12）に近いものです。
	//e3b0c442-98fc-1c14-9ddf-8fae5c57c0f3
	return fmt.Sprintf(`%x-%x-%x-%x-%x`, buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}
