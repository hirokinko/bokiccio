# Bokiccio Terraform

demo環境でoperatorが行う認証、入力値準備、実apply、IAP OAuth設定、browser確認は
[Demo environment operator runbook](DEMO_OPERATOR_RUNBOOK.md)を参照する。runbookには未実装段階の実行停止条件も記載している。

Google Cloud側の共通基盤とenvironmentを分離して管理する。Turso databaseは通常deploymentと分離したTerraform rootで管理し、
token payloadはTerraformへ渡さず、`scripts/bootstrap-turso-token`からSecret Managerへ直接登録する。

## Bootstrap

`bootstrap`はlocal stateから開始し、次を作成する。

- environment state用GCS bucket（Object Versioning、uniform access、public access prevention）
- environment間で共有するDocker image用Artifact Registry repository
- environment専用のpayloadを持たないSecret Manager secret
- environment deployment用service accountと必要resourceへの権限
- Cloud Run、Cloud Build、Artifact Registry、Secret Manager、IAPなどのAPI

Cloud Build service agentにはdeployment service accountの短期credentialを発行する権限だけを付与する。deployment service accountは
Cloud Loggingへの出力、container repositoryへのpush、state objectの更新、対象Turso secretの参照・IAM policy更新、および
environmentのCloud Run・runtime service account・IAP構成に必要な権限だけを持つ。
Secret Manager Adminは付与せず、対象secretのAccessorと`getIamPolicy` / `setIamPolicy`だけを持つcustom roleを使用する。

```sh
cp infra/terraform/bootstrap/terraform.tfvars.example \
  infra/terraform/bootstrap/terraform.tfvars
$EDITOR infra/terraform/bootstrap/terraform.tfvars

terraform -chdir=infra/terraform/bootstrap init
terraform -chdir=infra/terraform/bootstrap plan -out=bootstrap.tfplan
terraform -chdir=infra/terraform/bootstrap apply bootstrap.tfplan
```

`terraform.tfvars`、plan、stateはprivate artifactでありGitへ追加しない。bootstrap stateはstate bucket自身を管理するため、
作成後もlocal stateを暗号化されたprivate storageへbackupする。environment rootはbootstrap outputのbucketをGCS backendとして使う。

Application Default Credentialsまたはservice account impersonationを使用し、credential key fileをrepositoryへ置かない。
secret payload、Turso token、個人Google Accountはbootstrap variableへ渡さない。secret IDは`environment_id`から
`bokiccio-<environment_id>-turso-token`として生成する。

## Environment module

`modules/bokiccio_environment`は1 environment分のruntime service account、Cloud Run v2 service、対象secret access、
IAP service agentだけを持つauthoritative invoker binding、利用者のauthoritative IAP bindingを管理する。
containerはsha256 digest、secretは数値versionだけを受け付け、Turso token payloadはTerraformへ渡さない。

初回runtime service account作成ではresource単位の`roles/iam.serviceAccountUser`とsecret accessorの伝播を30秒待ってから
Cloud Run serviceを作成する。project全体のact-as権限へ広げず、IAM bindingのidentityが変わった場合だけ待機を再実行する。

`environment_id`はruntime identity、`service_name`はendpoint・IAP audience・originへ使う。service名を変更してもruntime identityと
secret参照は変わらない。`deletion_protection`はenvironment rootが明示し、有効なserviceをrenameまたは削除する場合は、先に
別applyで無効化してから置換planを確認する。

## Turso database

`modules/turso_database`はdatabaseだけを管理し、database token resourceを作成しない。demo rootはdatabase名を
`bokiccio-<environment_id>`から導出し、Turso provider `jpedroh/turso` v1.2.0とchecksumをlock fileで固定する。
databaseには`prevent_destroy`を設定しているため、退役時は保護解除を明示した変更が必要になる。

Turso管理用API tokenは`sensitive`かつ`ephemeral`なvariableからprovider設定だけへ渡す。`.tfvars`、plan、state、outputへ
tokenを保存しない。実行中は`TF_LOG`を設定しない。

```sh
cp infra/terraform/environments/demo/turso/backend.hcl.example \
  infra/terraform/environments/demo/turso/backend.hcl
cp infra/terraform/environments/demo/turso/terraform.tfvars.example \
  infra/terraform/environments/demo/turso/terraform.tfvars
$EDITOR infra/terraform/environments/demo/turso/backend.hcl
$EDITOR infra/terraform/environments/demo/turso/terraform.tfvars

read -r -s 'TF_VAR_turso_api_token?Turso API token: '
export TF_VAR_turso_api_token
printf '\n'

terraform -chdir=infra/terraform/environments/demo/turso init \
  -backend-config=backend.hcl
terraform -chdir=infra/terraform/environments/demo/turso plan \
  -out=turso.tfplan
terraform -chdir=infra/terraform/environments/demo/turso apply turso.tfplan

unset TF_VAR_turso_api_token
```

`backend.hcl`、`terraform.tfvars`、saved planはGit ignore対象である。tokenはshell historyへ書かず、planとapplyの両方が完了したら
直ちにunsetする。

database作成後、専用CLIはTurso CLIが生成したfull-access tokenから改行を除き、pipeでSecret Managerのstdinへ直接渡す。
tokenを標準出力、command引数、一時fileへ置かず、成功時はcredential-free database URL、secret ID、数値versionだけをJSONで返す。

```sh
scripts/bootstrap-turso-token \
  --database bokiccio-demo \
  --database-url "$(terraform -chdir=infra/terraform/environments/demo/turso output -raw database_url)" \
  --project replace-with-google-cloud-project-id \
  --secret bokiccio-demo-turso-token
```

Turso database tokenは個別に失効できない。Secret Manager登録が失敗した場合は再実行せず、secret version metadataを確認する。
tokenが生成されたものの保存されなかった場合、`turso db tokens invalidate <database> --yes`でdatabaseの全tokenを失効してから、
影響を受けるtokenをすべて再作成する。

## Demo application

`environments/demo/application`はGoogle Cloud側のenvironment moduleを呼び出すdemo専用rootである。用途に固定できる
`environment_id = "demo"`、scale-to-zero、最大1 instance、削除保護をdefaultとし、Cloud Run `service_name`、個人Google Account、
project、database URL、固定secret version、image digestの実値はtracked fileへ置かない。

```sh
cp infra/terraform/environments/demo/application/backend.hcl.example \
  infra/terraform/environments/demo/application/backend.hcl
cp infra/terraform/environments/demo/application/terraform.tfvars.example \
  infra/terraform/environments/demo/application/terraform.tfvars
$EDITOR infra/terraform/environments/demo/application/backend.hcl
$EDITOR infra/terraform/environments/demo/application/terraform.tfvars

terraform -chdir=infra/terraform/environments/demo/application init \
  -backend-config=backend.hcl
```

`terraform.tfvars`は通常deploymentとIAP利用者変更の両方で使うenvironment inputの正である。個人Google Accountは
`user:<email>`形式で記録し、Gitへ追加しない。`container_image`はこのfileへ固定せず、通常deploymentではCloud Buildが新digestを、
IAP利用者変更ではTerraform stateの`container_image` outputをCLI variableとして渡す。これによりaccess変更が旧imageを再deployしない。
初回plan/applyはmigration済みのdigestが用意されるまで実行しない。

## Cloud Build deployment

`cloudbuild.deploy.yaml`はoperatorが明示的にsubmitし、test、image build/push、digest解決、migration、Terraform apply、
IAP境界確認を直列実行する。custom deployment service accountを使用し、logはCloud Loggingだけへ保存する。
migrationが失敗すると後続stepは実行されない。

local source archiveはbootstrapで作成したprivate state bucketの`cloud-build-source/` prefixへstageする。custom deployment service accountは
このbucketへenvironment state更新用の限定権限を既に持つため、Google管理のdefault source bucketへproject-wide権限を追加しない。

Turso tokenはSecret Managerの固定versionから`secretEnv`へ注入し、command引数やsourceへ含めない。`.gcloudignore`は
Git管理外fileを原則除外し、secret payloadを持たないdemo application `terraform.tfvars`だけを明示的にsource stagingへ含める。
このfileはCloud Build内のTerraformとローカルで行うIAP利用者変更で共用する。

実行commandと必要substitutionは
[Demo environment operator runbook](DEMO_OPERATOR_RUNBOOK.md#5-cloud-build-deployment)を参照する。
