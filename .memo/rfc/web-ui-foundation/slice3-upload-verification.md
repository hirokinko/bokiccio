# Web UI Slice 3 upload verification

**Result:** Passed

**Confirmed:** 2026-08-12、local automated verificationとCloud Run production smoke確認で問題なし

## 対象

Web UI foundation RFCのSlice 3として、browserからのnormalized input v1 JSON uploadを確認した。

確認範囲:

- `POST /ui/imports`
- `POST /en/ui/imports`
- multipart/form-data decode
- 単一file制約、10 MiB file limit、request全体limit
- valid input upload後のrun detail redirect
- record-level errorを含むpartial outcomeの保存と表示
- invalid JSON/schema、missing file、extra field、multiple files、query string、oversize、unsupported media type
- filename、file content、SQL error、credentialをHTML responseへ出さないprivate-safe error
- Cloud Run direct IAP配下のsame-origin mutation
- `BOKICCIO_EXTERNAL_ORIGIN`のcomma-separated multiple HTTPS origins
- `Referrer-Policy: same-origin`によるbasic form POSTの`Origin: null`回避
- UI routeのsecurity errorはHTML、API routeのsecurity errorはJSONのまま維持

## local verification

repository rootで以下を実行し、すべてexit status `0`を確認した。

```console
gofmt -w internal/webapp/security.go internal/webapp/security_test.go internal/webprod/config_test.go internal/webprod/handler.go internal/webprod/handler_test.go internal/webui/handler.go internal/webui/handler_test.go internal/webui/messages.go
go test ./...
go test -race ./...
go vet ./...
```

追加確認:

- `internal/webui/handler_test.go`でupload success、partial outcome、invalid input、oversize、storage failureを確認。
- `internal/webprod/handler_test.go`でUIのorigin failureが`text/html; charset=utf-8`、APIのorigin failureが`application/json; charset=utf-8`であることを確認。
- `internal/webapp/security_test.go`で複数external originの許可と不正origin拒否を確認。

## production smoke verification

Cloud Run service `bokiccio`で、次を手動確認した。

- `https://bokiccio-r2eub5xl4q-an.a.run.app/`と`https://bokiccio-391222912924.asia-northeast1.run.app/`の両方からUIにアクセスできる。
- upload formからnormalized JSONを送信して403にならず、run detailへ遷移する。
- 403発生時もUI routeではJSONだけの画面にならず、BokiccioのHTML error pageとして表示される。

## 残作業

本Slice内のblocking issueはない。revision編集、approval UI、Tackler journal upload、provider固有format uploadは本RFCの範囲外であり、後続Sliceまたは別RFCで扱う。
