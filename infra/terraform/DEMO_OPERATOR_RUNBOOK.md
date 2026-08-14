# Demo environment operator runbook

この手順書は、個人データを含まないBokiccio demo environmentを既存Google Cloud projectと専用Turso databaseへ構築するときに、
operatorが行う作業をまとめる。repository内の実装・testは開発作業、Google CloudとTursoへの認証、課金対象projectの選択、
個人Google Accountの指定、実環境へのapplyとbrowser確認はoperator作業として扱う。

## 現在の状態

2026-08-14時点では、次の構成まで実装済みである。

- Secret Manager secret containerを含むGoogle Cloud bootstrap
- Cloud Run environment module
- Turso database用Terraform root
- database tokenをSecret Managerへ直接登録するbootstrap CLI
- demo application root

Cloud Build deploymentを含むrepository側の構成は実装済みである。Google Cloud bootstrap、Turso database apply、token bootstrapは
完了している。demo applicationの実値準備、Cloud Buildの実行、IAP OAuth設定、browser確認は未実施である。

## 秘密情報の禁止事項

次の値をrepository、`.tfvars`、saved plan、Terraform state、issue、chat、作業logへ記録しない。

- Turso管理用API token
- Turso database token
- IAP custom OAuth client secret
- service account key JSON

`.mise.local.toml`は既存のローカル確認用途だけに使い、demo構築用credentialの保管場所にはしない。
Terraform実行中は`TF_LOG`を有効にしない。tokenをshellのcommand引数へ直接書かず、作業後はshellへ一時設定したtokenをunsetする。

## Operatorが用意する値

実値はGit管理外のenvironment専用`.tfvars`へ記録する。tokenやOAuth client secretは記録しない。

| 項目 | 条件 | 秘密情報 |
| --- | --- | --- |
| Google Cloud project ID | 既存の課金有効project | No |
| region | Cloud RunとArtifact Registryで使用するregion | No |
| `environment_id` | supporting resourceを識別する安定した名前 | No |
| `service_name` | Cloud Run命名規則を満たす任意の名前 | No |
| state bucket名 | globally uniqueな3〜63文字 | No |
| Artifact Registry repository ID | environment間で共有するrepository | No |
| Turso organization slug | demo databaseを作るorganization | No |
| Turso group名 | 作成済みのgroup | No |
| Turso database名 | 個人用databaseと異なる名前 | No |
| IAP principal | `user:<個人Google Accountのemail>` | Privateだがsecretではない |
| Turso管理用API token | organization scopedを推奨 | Yes |

`environment_id`とTurso database名はCloud Runの`service_name`を変更しても変えない。

## 1. ローカルtoolとGoogle Cloud認証

repository rootでTerraformを用意する。

```sh
mise install
mise exec -- terraform version
gcloud version
turso --version
```

Google Cloud CLIとApplication Default Credentialsは別のcredentialであるため、両方へoperatorのGoogle Accountでloginする。

```sh
gcloud auth login
gcloud auth application-default login
gcloud config set project <PROJECT_ID>
gcloud auth list
gcloud config get project
```

service account key fileは作らない。別projectが選択されている場合はTerraform操作へ進まない。

## 2. Tursoの準備

Turso CLIへloginし、demo databaseの作成先organizationとgroupが存在することを確認する。既存の個人用databaseをdemo用として指定しない。

```sh
turso auth login
```

Terraform provider用には、対象organizationだけへscopeした管理用API tokenを使う。tokenは発行時にしか表示されないため、
必要ならローカルのpassword managerへ保管し、repository fileへ貼り付けない。

```sh
turso auth api-tokens mint bokiccio-demo-terraform --org <TURSO_ORGANIZATION>
```

Terraform rootはこの値を`sensitive`かつ`ephemeral`な入力として受け取る。通常のCloud Build deploymentには管理用API tokenを渡さない。

## 3. Google Cloud bootstrap

1. repositoryのexampleからGit管理外のbootstrap `.tfvars`を作る。
2. project、region、`environment_id`、state bucket、Artifact Registryの値を設定する。
3. `terraform plan`を保存し、対象が選択したprojectだけであることを確認する。
4. state bucket、Artifact Registry、deployment service account、空のSecret Manager secret、必要APIだけが作られることを確認する。
5. deployment service accountにSecret Manager Adminが付与されていないことを確認する。必要なのは対象secretのAccessorと、
   runtime accessorを管理するための`getIamPolicy` / `setIamPolicy`だけである。
6. 確認したsaved planだけをapplyする。
7. bootstrap local stateを暗号化されたprivate storageへbackupする。

次のいずれかがplanへ現れた場合は停止する。

- 既存production resourceの変更または削除
- `allUsers`または`allAuthenticatedUsers`
- secret payload
- 想定外projectのresource
- state bucketまたはTurso databaseの削除

## 4. Turso databaseとdatabase token

1. exampleからGit管理外の`backend.hcl`と`terraform.tfvars`を作り、bootstrapで作成したbucket、専用state prefix、
   Turso organizationを設定する。database名は`environment_id`から`bokiccio-demo`として導出される。
2. password managerに保管した管理用API tokenを、shell historyへ残さないようzshのsilent promptからephemeral入力へ設定する。

   ```sh
   read -r -s 'TF_VAR_turso_api_token?Turso API token: '
   export TF_VAR_turso_api_token
   printf '\n'
   ```

3. remote backendを初期化し、saved planを作成する。

   ```sh
   mise exec -- terraform -chdir=infra/terraform/environments/demo/turso init \
     -backend-config=backend.hcl
   mise exec -- terraform -chdir=infra/terraform/environments/demo/turso plan \
     -out=turso.tfplan
   ```

4. planが新しい`bokiccio-demo` database 1件だけを作り、既存databaseを変更・削除しないことを確認する。管理用API tokenが
   planへ含まれていないことも確認する。
5. 確認済みsaved planだけをapplyし、credentialを含まないdatabase URLを確認する。

   ```sh
   mise exec -- terraform -chdir=infra/terraform/environments/demo/turso apply turso.tfplan
   unset TF_VAR_turso_api_token
   ```

6. 専用CLIでdatabase tokenをTerraform管理済みSecret Manager secretへ直接登録する。tokenはpipe内だけを通り、terminalへ表示されない。

   ```sh
   scripts/bootstrap-turso-token \
     --database bokiccio-demo \
     --database-url "$(mise exec -- terraform -chdir=infra/terraform/environments/demo/turso output -raw database_url)" \
     --project replace-with-google-cloud-project-id \
     --secret bokiccio-demo-turso-token
   ```

7. CLIが返したJSONのsecret IDと数値versionをGit管理外のdemo application `.tfvars`へ記録する。database URLはcredentialを含まない。
8. Secret Managerのmetadataでversionがenabledであることを確認する。payload自体は再表示しない。

bootstrap CLIがtoken文字列をterminalへ表示した場合、またはSecret Manager登録前に失敗した場合は後続作業へ進まない。
失敗時はsecret version metadataを先に確認し、tokenが保存されていなければ
`turso db tokens invalidate bokiccio-demo --yes`でdatabaseの全tokenを失効する。これは既存tokenもすべて無効にするため、
自動実行せず影響を確認してから行う。tokenを手作業で`.tfvars`へ移して続行しない。

## 5. Cloud Build deployment

`cloudbuild.deploy.yaml`は、test、image build/push、digest解決、database migration、Terraform plan/apply、IAP境界確認を直列実行する。
custom deployment service accountを使用し、logはCloud Loggingだけへ保存する。

1. exampleからGit管理外のdemo application `backend.hcl`と`terraform.tfvars`を作る。

   ```sh
   cp infra/terraform/environments/demo/application/backend.hcl.example \
     infra/terraform/environments/demo/application/backend.hcl
   cp infra/terraform/environments/demo/application/terraform.tfvars.example \
     infra/terraform/environments/demo/application/terraform.tfvars
   ```

2. `.tfvars`へproject、region、`environment_id`、`service_name`、database URL、secret ID、数値version、IAP principalを設定する。
   `container_image`は記録しない。通常deploymentはCloud Buildが新digestをCLI variableとして渡す。
3. `iap_principals`にoperator自身の`user:<email>`が含まれていることを確認する。
4. application backendを初期化する。

   ```sh
   mise exec -- terraform -chdir=infra/terraform/environments/demo/application init \
     -backend-config=backend.hcl
   ```

5. source staging対象を確認する。demo application `terraform.tfvars`が含まれ、bootstrap/Tursoの`.tfvars`、`backend.hcl`、state、plan、
   `.mise.local.toml`、`.memo`が含まれないことを確認する。

   ```sh
   gcloud meta list-files-for-upload
   ```

6. 次のnon-secret substitutionを指定してCloud Buildを手動submitする。

   ```sh
   gcloud builds submit . \
     --project=<PROJECT_ID> \
     --region=<REGION> \
     --config=cloudbuild.deploy.yaml \
     --gcs-source-staging-dir=gs://<STATE_BUCKET>/cloud-build-source \
     --substitutions=_REGION=<REGION>,_ARTIFACT_REPOSITORY=<REPOSITORY_ID>,_TURSO_DATABASE_URL=<DATABASE_URL>,_TURSO_SECRET_ID=<SECRET_ID>,_TURSO_SECRET_VERSION=<NUMERIC_VERSION>,_TF_STATE_BUCKET=<STATE_BUCKET>,_DEPLOYMENT_SERVICE_ACCOUNT=<DEPLOYMENT_SERVICE_ACCOUNT_EMAIL>
   ```

   substitutionにtoken payloadやOAuth client secretを渡さない。`_TURSO_SECRET_VERSION`は`latest`ではなく数値versionとする。
   source archiveはprivate state bucketの専用prefixへstageし、custom deployment service accountへdefault Cloud Build source bucketの
   project-wide Storage権限を追加しない。
7. test、image build/push、database migration、Terraform plan/apply、IAP境界確認がこの順で成功したことを確認する。
   IAP境界確認は未認証の`/livez`が`302`、`401`、`403`のいずれかとなり、publicな`2xx`を返さないことを検査する。
   applicationの起動自体はCloud Run startup probeが`/livez`の成功を確認する。
8. build結果からimmutable image digestを記録し、Terraformの`container_image` outputが同じdigestであることを確認する。
9. migration失敗時にTerraform applyが実行されていないことを確認する。push済みだが未deployのimageはそのまま調査対象として残し、
   同じbuildを無条件に再実行しない。

source repository triggerやmigration用Cloud Run Jobは作成しない。Cloud Buildへ送るapplication `terraform.tfvars`はprivate inputだが
secret payloadを持たず、Terraform planと同様にIAP principalがproject内のbuild logへ現れる可能性がある。

## 6. 個人Google Account向けIAP OAuth設定

個人Google Accountまたはorganization外accountを許可するため、projectごとにcustom OAuth clientのone-time設定が必要になる。
Cloud Run serviceの初回作成後、次の操作をGoogle Cloud Consoleで行う。

1. Cloud Runで対象serviceを開き、**Security**を選ぶ。
2. IAPの**Edit policy**から**Configure in IAP**を開く。
3. OAuth consent screenを構成し、Audienceを**External**にする。
4. 初期構築では**Auto generate credentials**を使用する。
5. 設定を保存する。

この操作ではIAP利用者をConsoleから追加しない。利用者listの正はGit管理外`.tfvars`の`iap_principals`であり、Terraform applyで管理する。
手動でOAuth clientを作る場合もclient secretをrepository、`.tfvars`、Terraformへ渡さない。

## 7. Browser確認とdemo data

1. 許可した個人Google Accountの通常windowでservice URLを開き、IAP login後に画面が表示されることを確認する。
2. 未許可accountの別profileまたはsecret windowではapplication contentを取得できないことを確認する。
3. demo databaseが空であり、個人用databaseやbackupを接続していないことを再確認する。
4. uploadがenabledの間に`samples/normalized-input-upload-v1.json`だけを取り込む。
5. 必要な承認とreporting設定をdemo上で行う。
6. operator CLI `bokiccio settings set --file-upload-enabled=false`でuploadを無効化する。
7. 日本語・英語画面から両upload formが消え、3つのimport routeが`403`となることを確認する。
8. 検索、entry詳細、revision、approval、export、reportが引き続き利用できることを確認する。

## 8. IAP利用者の追加・削除

1. Git管理外のdemo application `.tfvars`にある`iap_principals`を更新する。
2. application deploymentを起動せず、stateにある現在のimage digestを同じplanへ明示的に渡す。

   ```sh
   BOKICCIO_DEPLOYED_IMAGE="$(mise exec -- terraform -chdir=infra/terraform/environments/demo/application output -raw container_image)"
   mise exec -- terraform -chdir=infra/terraform/environments/demo/application plan \
     -out=iap-access.tfplan \
     -var="container_image=${BOKICCIO_DEPLOYED_IMAGE}"
   ```

3. 差分がIAPのauthoritative bindingだけであることを確認し、確認済みplanだけをapplyする。

   ```sh
   mise exec -- terraform -chdir=infra/terraform/environments/demo/application apply iap-access.tfplan
   unset BOKICCIO_DEPLOYED_IMAGE
   ```

4. apply後、次のread-only commandで結果を確認する。

```sh
gcloud iap web get-iam-policy \
  --region=<REGION> \
  --resource-type=cloud-run \
  --service=<SERVICE_NAME>
```

Consoleや`gcloud iap web add-iam-policy-binding`による直接変更は通常運用にしない。operator自身が唯一の管理経路である場合、
自身をlistから削除しない。

## 完了記録

実環境確認ではsecret値を記録せず、次だけをRFC verificationへ残す。

- 実行日時と対象environment ID
- Terraform plan/applyの成否とresource件数
- image digest
- database名とcredential-free URL
- Secret Manager secret IDと数値version
- migration結果
- IAP allow/deny結果
- upload disabledとread機能の結果
- rollbackまたは停止が必要になった場合の段階

## 参考資料

- [Application Default Credentials](https://docs.cloud.google.com/docs/authentication/set-up-adc-local-dev-environment)
- [Cloud Run direct IAP](https://docs.cloud.google.com/run/docs/securing/identity-aware-proxy-cloud-run)
- [IAP custom OAuth client](https://docs.cloud.google.com/iap/docs/custom-oauth-configuration)
- [Turso Platform API authentication](https://docs.turso.tech/api-reference/authentication)
- [Turso database token creation](https://docs.turso.tech/cli/db/tokens/create)
- [Secret Manager best practices](https://docs.cloud.google.com/secret-manager/docs/best-practices)
