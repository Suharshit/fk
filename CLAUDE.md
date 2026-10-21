# fk — notes for Claude

fk is a Go tool that compresses and converts PDF, PNG and JPG files: an interactive CLI (`fk`), a local web terminal (`fk serve`) and a server web app/API (`fk api`).

## DevOps pipeline

- The pipeline follows **docs/DEVOPS_PIPELINE_TEMPLATE.md**. Treat it as the project's DevOps playbook: when adding or changing pipeline stages, follow its stage structure, owner calls and ✅ verify checks.
- Decisions for this project are in **pipeline.config.yml**. Keep it in sync when a decision changes.
- The fk-specific guide, demo runbook and syllabus mapping are in **docs/PIPELINE.md**.
- Ownership rule: Ansible installs and deploys (push). Puppet owns ongoing config: the nginx LB config, keeping the fk service running, and the cleanup cron (pull). Don't make both tools manage the same file.

## Commands

- `make test`, `make lint`, `make build`, `make run-api`
- Infra: `make up` / `make down` (AWS), `make vagrant-up` / `make vagrant-configure` (local)
- Before pushing: `gofmt -l .` must print nothing; `go vet ./...` and `go test ./...` must pass. CI also runs `terraform fmt -check`, `ansible-lint` and `puppet-lint --fail-on-warnings`.

## Conventions

- Release artifact name: `fk-linux-amd64`; version injected with `-ldflags "-X main.version=<tag>"`.
- fk api must never expose a shell or arbitrary file access. Each request works in its own temp folder that is deleted afterwards.
- Host names: lb-1, app-1..N, ops-1. Groups: lb, app, ops.
