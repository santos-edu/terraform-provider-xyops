# Terraform Provider for xyOps

[![CI](https://github.com/santos-edu/terraform-provider-xyops/actions/workflows/test.yml/badge.svg)](https://github.com/santos-edu/terraform-provider-xyops/actions/workflows/test.yml)

Provider for [xyOps](https://github.com/pixlcore/xyops), a server scheduler and monitor. The same configuration works in Terraform and OpenTofu. Published as [santos-edu/xyops](https://registry.terraform.io/providers/santos-edu/xyops) on the Terraform Registry and the [OpenTofu Registry](https://search.opentofu.org/provider/santos-edu/xyops/latest). Requires Terraform >= 1.3 or OpenTofu, a running xyOps instance, and an API key. Argument reference is in [`docs/`](docs/index.md).

## Configuration

`endpoint` and `api_key` are required in the provider block or via `XYOPS_ENDPOINT` and `XYOPS_API_KEY`.

```hcl
terraform {
  required_providers {
    xyops = {
      source  = "santos-edu/xyops"
      version = "~> 0.1"
    }
  }
}

provider "xyops" {
  endpoint = "https://xyops.example.com"
}
```

## Resources and data sources

Resources: `xyops_alert`, `xyops_api_key`, `xyops_bucket`, `xyops_category`, `xyops_channel`, `xyops_config`, `xyops_event`, `xyops_group`, `xyops_monitor`, `xyops_plugin`, `xyops_role`, `xyops_secret`, `xyops_server`, `xyops_tag`, `xyops_webhook`, `xyops_workflow`.

Data sources: `xyops_alerts`, `xyops_jobs`, `xyops_servers`. They call the xyOps `search_*` actions. `query` uses xyOps index syntax (`*` lists everything). `offset` defaults to 0 and `limit` defaults to 100.

`xyops_secret.value`, webhook `url`, `headers` and `body`, and `xyops_api_key` `key` and `plain_key` are sensitive. Terraform redacts them in CLI output. xyOps returns these values in clear text, so state stores them in clear text. Protect the state the same way you protect the API key.

`xyops_server` adopts a server the agent already registered. It does not create or delete it. Destroy, or removing the block, only stops managing it. Omitting `enabled` keeps the current value. `id` allows lowercase letters, digits, and underscore.

`xyops_config` is a singleton, cannot be imported, and destroy only clears `job_universal_actions`. Use `prevent_destroy`.

Every other resource imports by xyOps id. Optional JSON attributes stay `null` on import, so set them in configuration when Terraform should manage them.

```bash
terraform import xyops_alert.high_cpu high_cpu
```

JSON attributes are compared by structure. Write them with `jsonencode()`. Reference lists keep the order xyOps returns.

## Development

Go 1.27.2. `.tool-versions` pins Terraform 1.17.1 and OpenTofu 1.13.0. `make compose-up` starts xyOps, `make build` writes the provider binary in the repository root, and `make testacc` runs the acceptance tests. Point `dev_overrides` for `santos-edu/xyops` at that directory. `make docs` regenerates `docs/`.

Set `TF_ACC_TERRAFORM_PATH` to choose which CLI the acceptance tests run. CI runs them once with Terraform 1.17.1 and once with OpenTofu 1.13.0. For OpenTofu it also sets `TF_ACC_PROVIDER_HOST=registry.opentofu.org` and `TF_ACC_PROVIDER_NAMESPACE=hashicorp`.

## Release

Set the version in `CHANGELOG.md`, add the `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` repository secrets, and push a `vX.Y.Z` tag. GoReleaser opens a draft GitHub release. Publish the draft. The Terraform Registry and the OpenTofu Registry both index that release. `santos-edu/xyops` and its signing key are already registered, so a new version does not need another submission.

Submit a new public key only when the signing key changes, using the [OpenTofu signing key form](https://github.com/opentofu/registry/issues/new?assignees=&labels=provider-key%2Csubmission&template=provider_key.yml&title=Provider+Key%3A+). Namespace `santos-edu`, provider `xyops`. The registry only processes that form in the browser.
