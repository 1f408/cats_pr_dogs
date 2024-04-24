# ファイル構成

レポジトリ内のファイル構成の概略です。
個別のファイルについては説明していません。

```
/
+-- conf/             # デフォルト設定ファイル置き場(カスタム設定ファイルで差し替え可能)
|   +-- etc/              # 設定ファイル
|   +-- gen/          # Go generate用コード
|   +-- lib/          # ライブラリ置き場
|   |   +-- app_mpl/      # アプリ用HTML template置き場
|   |   +-- icon/         # template用ICONデータ置き場
|   |   +-- tmpl/         # HTML templateパーツ置き場
|   |
|   +-- www/
|       +-- css/          # スタイルファイル置き場
|       +-- font/         # fontsデータ置き場
|       +-- js/           # JavaScriptコード置き場
|       +-- lib/          # JavaScript外部ライブラリ置き場
|
+-- docs/             # ドキュメント
+-- icons/            # ビルド用ICONソース置き場
+-- lorca/            # カスタム版Lorcaライブラリ
+-- src/              # cats_pr_dogsソースコード
+-- *.bash            # build用bashスクリプト
+-- *.sh              # build用shellスクリプト
+-- go.*              # go.mod関連ファイル
```
