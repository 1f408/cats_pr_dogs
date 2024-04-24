[cats\_dogs]: https://github.com/1f408/cats_dogs

# cat\_pr\_tmpl(CatPrTmpl)

[cats\_dogs]のcat\_tmplviewと同じMarkdown処理を行うプレビューアプリです。
アカウント名でのコンテンツ出し分けに対応した、Markdownテンプレートファイルとして、ファイルを表示します。  
純粋なMarkdownファイルを表示したい場合は、[cat\_pr\_md](cat_pr_md.md)を利用してください。

基本的には、[cats\_dogs]のcat\_tmplviewと同じ表示をしますが、
CatUIの機能は、(サーバで動かさないと意味がないので)安全のため無効化しています。

カスタマイズについては、[カスタマイズ方法](customize.md)を参照してください。

## 表示アカウントの変更方法

アカウント名の入力欄が画面の左上に追加されます。
この入力欄にアカウント名を入力することで、そのアカウントの表示に変更することが出来ます。

ただし、事前に、`etc/usermap.conf`でアカウントの設定をする必要があります。  
方法については、[カスタマイズ方法](customize.md)を参照してください。
