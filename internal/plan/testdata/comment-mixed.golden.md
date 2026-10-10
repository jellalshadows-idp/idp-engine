<!-- idp-plan -->
## IDP plan

| Stack | Create | Update | Replace | Delete |
|---|---:|---:|---:|---:|
| <code>github</code> | 1 | 1 | 1 ⚠️ | 1 ⚠️ |

<details><summary><code>github</code>: +1 ~1 -1 ±1</summary>

| Action | Address |
|---|---|
| update | <code>module.group_platform.terraform_data.member["bob"]</code> |
| ⚠️ delete | <code>module.group_platform.terraform_data.member["carol"]</code> |
| create | <code>module.group_platform.terraform_data.member["dave"]</code> |
| ⚠️ replace | <code>terraform_data.replaced</code> |

</details>

> [!WARNING]
> This plan deletes or replaces resources. The reconcile run after merge will wait for approval in the `idp-approval` environment.

Commit `0123456` · [workflow run](https://github.com/acme/idp-claims/actions/runs/1)
