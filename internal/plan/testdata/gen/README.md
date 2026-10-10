# Plan fixtures

`creates.json` and `mixed.json` are real `tofu show -json` outputs from OpenTofu 1.12.6 (spec §8.2: plan fixtures are real tool output, never hand-written). `terraform_data` is built into OpenTofu, so no provider and no network are needed.

- `creates.json` is step 1 planned against empty state: four creates.
- `mixed.json` is step 2 planned against step 1's state:
  - `alice` is a no-op;
  - `bob` is an update;
  - `carol` is a delete;
  - `dave` is a create;
  - `terraform_data.replaced` is a replace.

Regenerate them from the repo root in Git Bash:

```bash
TOFU="$HOME/.idp/tools/tofu-1.12.6/tofu.exe"
W="$(mktemp -d)" && cp -r internal/plan/testdata/gen/. "$W/" && ROOT="$PWD" && cd "$W" \
  && cp step1.tf.txt main.tf && "$TOFU" init -input=false >/dev/null \
  && "$TOFU" plan -input=false -out=p1.bin >/dev/null && "$TOFU" show -json p1.bin > creates.json \
  && "$TOFU" apply -input=false p1.bin >/dev/null \
  && cp step2.tf.txt main.tf && "$TOFU" plan -input=false -out=p2.bin >/dev/null \
  && "$TOFU" show -json p2.bin > mixed.json \
  && cp creates.json mixed.json "$ROOT/internal/plan/testdata/" && cd "$ROOT"
```
