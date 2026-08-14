# Bokiccio Terraform

Google Cloud側の共通基盤とenvironmentを分離して管理する。Turso databaseとtoken payloadはTerraformの管理対象外とし、
既存のSecret Manager secret IDだけを参照する。

## Bootstrap

`bootstrap`はlocal stateから開始し、次を作成する。

- environment state用GCS bucket（Object Versioning、uniform access、public access prevention）
- Docker image用Artifact Registry repository
- environment deployment用service accountと必要resourceへの権限
- Cloud Run、Cloud Build、Artifact Registry、Secret Manager、IAPなどのAPI

Cloud Build service agentにはdeployment service accountの短期credentialを発行する権限だけを付与する。deployment service accountは
Cloud Loggingへの出力、container repositoryへのpush、state objectの更新、対象Turso secretの参照・IAM管理、および
environmentのCloud Run・runtime service account・IAP構成に必要な権限だけを持つ。

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
secret payload、Turso token、個人Google Accountはbootstrap variableへ渡さない。

## Environment module

`modules/bokiccio_environment`は1 environment分のruntime service account、Cloud Run v2 service、対象secret access、
IAP service agentだけを持つauthoritative invoker binding、利用者のauthoritative IAP bindingを管理する。
containerはsha256 digest、secretは数値versionだけを受け付け、Turso token payloadはTerraformへ渡さない。

`environment_id`はruntime identity、`service_name`はendpoint・IAP audience・originへ使う。service名を変更してもruntime identityと
secret参照は変わらない。`deletion_protection`はenvironment rootが明示し、有効なserviceをrenameまたは削除する場合は、先に
別applyで無効化してから置換planを確認する。
