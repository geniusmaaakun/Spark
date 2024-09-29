package auth

import (
	"crypto/sha512"
	"encoding/hex"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"crypto/sha256"
	"net/http"

	"github.com/gin-gonic/gin"
)

/*
Ginフレームワークを使ったWebアプリケーションにおけるBasic認証の実装です。
ユーザーの認証情報を検証し、適切なハッシュアルゴリズムを使ってパスワードを確認する仕組みが組み込まれています。


基本的な流れ
BasicAuth関数は、Ginのミドルウェアとして動作します。この関数はユーザー名とパスワードを検証し、成功すればそのリクエストを許可し、失敗すればHTTPステータスコード401でリクエストを拒否します。
複数のハッシュアルゴリズムに対応しており、パスワードが平文（plain）、SHA256、SHA512、Bcryptのいずれかで保存されている場合、それぞれ適切なハッシュアルゴリズムで検証します。
*/

// 認証アルゴリズムの定義
/*
algorithmsマップは、複数のパスワードハッシュアルゴリズムを処理する関数を定義しています。
plain: パスワードが平文で保存されている場合、入力されたパスワードと比較します。
sha256: SHA-256アルゴリズムでパスワードをハッシュ化し、保存されているハッシュ値と比較します。
sha512: SHA-512アルゴリズムでパスワードをハッシュ化し、保存されているハッシュ値と比較します。
bcrypt: Bcryptで保存されたパスワードを、bcrypt.CompareHashAndPasswordを使って検証します。


複数のハッシュアルゴリズム（plain、sha256、sha512、bcrypt）に対応し、認証情報を検証。
正規表現を使ってパスワードに指定されたアルゴリズムを判別し、適切な方法で認証。
リクエストごとに、正しいユーザー名とパスワードが提供されたかを確認し、正しくない場合は401 Unauthorizedでアクセスを拒否。
*/
var algorithms = map[string]func(string, string) bool{
	`plain`: func(hashed, password string) bool {
		return hashed == password
	},
	`sha256`: func(hashed, password string) bool {
		hash := sha256.Sum256([]byte(password))
		return hashed == hex.EncodeToString(hash[:])
	},
	`sha512`: func(hashed, password string) bool {
		hash := sha512.Sum512([]byte(password))
		return hashed == hex.EncodeToString(hash[:])
	},
	`bcrypt`: func(hashed, password string) bool {
		return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) == nil
	},
}

/*
**BasicAuth**は、ユーザーアカウント（accountsマップ）を基に、認証を行うミドルウェア関数を返す関数です。
accountsは、ユーザー名をキー、パスワードを値とするマップです。パスワードはハッシュ化されている場合もあります。
正規表現（regexp）を使って、パスワードが特定の形式で指定されているかどうかを確認します。形式は$algorithm$hashedPasswordという形で、どのアルゴリズムを使うかを指定します。
algorithm部分は、plain、sha256、sha512、bcryptのいずれかです。
パスワードがこの形式に合致する場合は、そのアルゴリズムに基づいて後でパスワードの検証が行われます。
**stdAccounts**には、ユーザー名をキーにして、どのアルゴリズムを使うかとハッシュされたパスワードのペア（cipher構造体）を保存します。
*/
func BasicAuth(accounts map[string]string, realm string) gin.HandlerFunc {
	type cipher struct {
		algorithm string
		password  string
	}
	if len(realm) == 0 {
		realm = `Authorization Required`
	}
	reg := regexp.MustCompile(`^\$([a-zA-Z0-9]+)\$(.*)$`)
	stdAccounts := make(map[string]cipher)
	for user, pass := range accounts {
		if match := reg.FindStringSubmatch(pass); len(match) > 0 {
			match[1] = strings.ToLower(match[1])
			if _, ok := algorithms[match[1]]; ok {
				stdAccounts[user] = cipher{
					algorithm: match[1],
					password:  match[2],
				}
				continue
			}
		}
		stdAccounts[user] = cipher{
			algorithm: `plain`,
			password:  pass,
		}
	}

	//リクエストごとの認証
	/*
		Basic認証の実行:

		c.Request.BasicAuth()を使って、リクエストからユーザー名とパスワードを取得します。もし、認証情報が無ければ、401 Unauthorizedを返して認証が必要なことをクライアントに伝えます。
		認証情報の検証:

		取得したユーザー名をもとに、stdAccountsから対応するパスワード情報を取得します。さらに、そのユーザーが設定されているアルゴリズムに基づいてパスワードをチェックします。適切なアルゴリズム関数を使って、入力されたパスワードと保存されたハッシュを比較します。
		成功した場合:

		認証に成功した場合、ユーザー名をコンテキストにセットして、その後の処理を続行します。
		失敗した場合:

		認証に失敗した場合は、401 Unauthorizedを返し、クライアントに再度認証を促します。
	*/
	return func(c *gin.Context) {
		user, pass, ok := c.Request.BasicAuth()
		if !ok {
			c.Header(`WWW-Authenticate`, `Basic realm=`+realm)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if account, ok := stdAccounts[user]; ok {
			if check, ok := algorithms[account.algorithm]; ok {
				if check(account.password, pass) {
					c.Set(`user`, user)
					return
				}
			}
		}
		c.Header(`WWW-Authenticate`, `Basic realm=`+realm)
		c.AbortWithStatus(http.StatusUnauthorized)
	}
}
