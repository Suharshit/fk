# DevOps Pipeline Template

**CI → Terraform → Ansible → Puppet → Nagios**, for any project.

Template version 1.0 · Built for INT333 (DevOps Advance Configuration Management) · Ubuntu 24.04 targets

---

## How to use this file

Give this file to whoever is building the pipeline: yourself, a teammate, or an AI assistant such as Claude. Point them at your project repository and say:

> "Follow DEVOPS_PIPELINE_TEMPLATE.md for this project."

The template is built from **stages**. Each stage has the same parts:

| Part | Meaning |
|---|---|
| **Purpose** | Why the stage exists |
| **🟡 OWNER CALL** | Questions the builder **must ask the project owner** before doing anything in that stage. The owner can include, skip or customise the stage. |
| **Needs** | Stages that must run first |
| **Files** | Files to create, with templates to adapt |
| **Run** | Commands to execute |
| **✅ Verify** | Checks that prove the stage works. Don't move on until they pass. |
| **If skipped** | What changes in later stages when this one is skipped |

### Rules for the builder (human or AI)

1. **Start with Stage 0 (Intake).** Fill in `pipeline.config.yml` with the owner before writing any other file.
2. **Stop at every 🟡 OWNER CALL.** Ask the questions exactly, show the default, and **wait for an answer**. Never assume "yes". Record the answer in `pipeline.config.yml`.
3. **Replace every `__PLACEHOLDER__`** with the value from `pipeline.config.yml`. Placeholders use double underscores so they never clash with Jinja `{{ }}`, Terraform `${ }`, GitHub Actions `${{ }}` or Puppet `$var`.
4. **Adapt, don't copy blindly.** The templates are starting points. Change them to fit the project's language, runtime and ports.
5. **Run ✅ Verify after each stage.** Report the results to the owner before starting the next stage.
6. **Never put secrets in the repository**: no AWS keys, SSH private keys, tokens or passwords. Use Stage 9.
7. **When a stage is skipped,** follow its "If skipped" note so later stages still work.

### Stage map

```text
 S0 Intake ──► S1 App readiness ──► S2 Repo hygiene ──► S3 CI ──► S4 Release
                                                                     │
         ┌───────────────────────────────────────────────────────────┘
         ▼
 S5 Infrastructure (Terraform | Vagrant | existing servers)
         │  writes inventory
         ▼
 S6 Ansible  (install + deploy, push)  ──►  S7 Puppet (keep state, pull)
         │                                      │
         └──────────────►  S8 Nagios (monitor)  ◄┘
                                │
 S9 Secrets & security ─ applies to all stages
 S10 Orchestration (Makefile) · S11 Demo & teardown · S12 Presentation
```

| Stage | Tool | Optional? | Syllabus link |
|---|---|---|---|
| S0 Intake | — | No | — |
| S1 App readiness | your code | No (light version allowed) | Unit III: "Deploying a simple web application" |
| S2 Repo hygiene | Git | No | — |
| S3 CI | GitHub Actions | Yes | (supports all units) |
| S4 Release | GitHub Releases / GHCR | Yes | — |
| S5 Infrastructure | Terraform / Vagrant | Yes | Unit IV, Unit V (Vagrant), Practicals 1, 14, 15 |
| S6 Configuration & deploy | Ansible | Yes | Units V–VI, Practicals 11, 12, 13, 15 |
| S7 State enforcement | Puppet (OpenVox) | Yes | Units I–II, Practicals 2–6 |
| S8 Monitoring | Nagios | Yes | Unit III, Practicals 7–10 |
| S9 Secrets & security | — | No | Unit IV: security considerations |
| S10 Orchestration | Make | Yes | — |
| S11 Demo & teardown | — | Yes | — |
| S12 Presentation | — | Yes | CO1–CO6 |

---

## Stage 0: Intake

**Purpose:** collect every decision in one file, so the rest of the build is mechanical.

### 🟡 OWNER CALL S0

Ask the owner each question. Offer the default in brackets.

1. Project name, lowercase with no spaces? *[repo name]*
2. GitHub repository, as `owner/repo`?
3. Language and runtime? *(Go, Node.js, Python, Java, Rust, static site, other)*
4. What is the deployable artifact?
   - **(a) binary**: a single executable, such as Go or Rust
   - **(b) container**: a Docker image
   - **(c) source + runtime**: Node.js, Python or Java run from source or a jar
5. Commands, each of which may be "none":
   - Install dependencies
   - Lint
   - Test
   - Build
   - Run
6. Port the app listens on? *[8000]*
7. Health check path? *[/healthz]*
8. Optional functional test: an endpoint plus sample input that proves the app *really works*, not just that it's up. For example, upload a file and expect HTTP 200. *[none]*
9. Extra OS packages the app needs on servers? For example `ghostscript` or `libvips`. *[none]*
10. Where will it run?
    - **(a) AWS** with Terraform
    - **(b) local VMs** with Vagrant and VirtualBox
    - **(c) existing servers** whose IPs you provide
11. Which stages to include? Show the stage map and let the owner tick: S3 CI, S4 Release, S5 Infra, S6 Ansible, S7 Puppet, S8 Nagios, S10 Make, S11 Demo, S12 Presentation.
12. Number of app servers? *[2]* Use a load balancer? *[yes if 2 or more]*
13. Deadline or date of the demo? This decides how much to build if time is short.

### Files

**`pipeline.config.yml`** at the repo root is the single source of truth. Fill it in from the answers.

```yaml
# pipeline.config.yml — answers from Stage 0. Every later stage reads from here.
project:
  name: __APP_NAME__            # e.g. fk
  repo: __GITHUB_REPO__         # e.g. Suharshit/fk
  language: __LANGUAGE__        # go | node | python | java | rust | static
  artifact: __ARTIFACT_TYPE__   # binary | container | source
  artifact_name: __ARTIFACT_NAME__   # e.g. fk-linux-amd64, or ghcr.io/owner/app
commands:
  install: "__INSTALL_CMD__"
  lint: "__LINT_CMD__"
  test: "__TEST_CMD__"
  build: "__BUILD_CMD__"
  run: "__RUN_CMD__"            # how the server starts, e.g. "fk api --port 8000"
app:
  port: __APP_PORT__            # e.g. 8000
  health_path: __HEALTH_PATH__  # e.g. /healthz
  functional_check: "__FUNCTIONAL_CHECK__"   # e.g. "POST /api/compress sample.jpg -> 200", or "none"
  system_packages: [__SYSTEM_PACKAGES__]     # e.g. ghostscript
environment:
  target: __TARGET__            # aws | vagrant | existing
  aws_region: __AWS_REGION__    # e.g. ap-south-1 (Mumbai)
  app_count: __APP_COUNT__      # e.g. 2
  load_balancer: __LB__         # true | false
stages:                         # true = include, false = skip (owner's call)
  ci: true
  release: true
  infra: true
  ansible: true
  puppet: true
  nagios: true
  make: true
  demo: true
  presentation: true
config_owner: puppet            # who owns ongoing config files: puppet | ansible (see S7)
deadline: __DEADLINE__
```

### ✅ Verify

- [ ] Every `__PLACEHOLDER__` in `pipeline.config.yml` is filled in, or explicitly set to `none`.
- [ ] The owner has confirmed the `stages:` list.

---

## Stage 1: App readiness

**Purpose:** make the app easy to deploy, monitor and manage. Every later tool depends on these basics.

### 🟡 OWNER CALL S1

- "May I add a **health endpoint** (`__HEALTH_PATH__`) and a **version** output to the app?" *[yes]*
- "Should the app read its port and settings from **environment variables**?" *[yes]*
- If the app is a CLI or desktop tool with no server: "Should we add a small **server mode** so it can be deployed, for example an upload-and-process web API? Or should we deploy only a static page or docs?" *[server mode]*

### Checklist for the app

| Need | Why | Example |
|---|---|---|
| Listens on `0.0.0.0:$PORT` | So the load balancer can reach it | `PORT=8000` |
| `GET __HEALTH_PATH__` returns 200 and `ok` | Nagios and Ansible check it | `{"status":"ok"}` |
| `--version` or `GET /version` | Puppet's fact and Nagios can report what's running | `1.2.0` |
| Settings come from env vars, not hard-coded | Same build runs everywhere | `MAX_UPLOAD_MB=20` |
| Logs go to stdout/stderr | systemd's journal collects them | — |
| Shuts down cleanly on SIGTERM | Rolling deploys don't drop requests | — |
| Temporary files are cleaned up | Disk doesn't fill up; Puppet can also enforce a cleanup cron | — |
| Upload or input size limits | Safety on a public server | 413 if too big |
| **Never exposes a shell or arbitrary file access** | Security | — |

### ✅ Verify

```bash
__RUN_CMD__ &                                         # start locally
curl -fsS http://localhost:__APP_PORT____HEALTH_PATH__  # prints ok
kill %1
```

**If skipped:** later stages fall back to a TCP port check instead of an HTTP health check, which is weaker monitoring.

---

## Stage 2: Repo hygiene

**Purpose:** a clean repository that anyone can clone and build.

### 🟡 OWNER CALL S2

- "Use a `main` branch with short-lived feature branches merged through pull requests?" *[yes]*
- "Protect `main` so CI must pass before merging?" *[yes if S3 is included]*

### Files

**`.gitignore`**: add these lines to the language defaults.

```gitignore
# build output
dist/
# terraform
.terraform/
*.tfstate
*.tfstate.*
*.tfvars
!example.tfvars
crash.log
# ansible
*.retry
infra/ansible/inventory/hosts.ini
# secrets: never commit these
*.pem
.env
.vault_pass*
```

**Repository layout** (create only the folders for included stages):

```text
__APP_NAME__/
├── (application code)
├── pipeline.config.yml
├── Makefile                      # S10
├── Vagrantfile                   # S5, only when target is vagrant
├── .github/workflows/            # S3, S4
│   ├── ci.yml
│   └── release.yml
└── infra/
    ├── terraform/                # S5
    │   ├── main.tf  variables.tf  outputs.tf  inventory.tftpl  example.tfvars
    │   └── modules/{network,compute}/
    ├── ansible/                  # S6
    │   ├── ansible.cfg  site.yml  deploy.yml
    │   ├── inventory/  group_vars/
    │   └── roles/{common,app,lb,nrpe,nagios,puppet_server,puppet_agent}/
    ├── puppet/                   # S7
    │   ├── manifests/site.pp
    │   ├── data/common.yaml  hiera.yaml
    │   └── modules/__APP_NAME__/
    └── nagios/                   # S8
        ├── conf.d/
        └── plugins/check_app.py
```

### ✅ Verify

- [ ] A fresh `git clone` followed by the build command works on a clean machine.
- [ ] `git status` shows no secrets or build output.

---

## Stage 3: Continuous Integration (GitHub Actions)

**Purpose:** every push is tested automatically, including the infrastructure code.

### 🟡 OWNER CALL S3

- "Include CI?" *[yes]*
- "Also lint the infrastructure code (Terraform, Ansible, Puppet, Nagios plugin) in CI?" *[yes]*
- "Run on every push, or only on pull requests to `main`?" *[both]*

### Files

**`.github/workflows/ci.yml`**

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

jobs:
  app:
    name: App — lint, test, build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      # __SETUP_STEP__ — pick one from the table below
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Install
        run: __INSTALL_CMD__
      - name: Lint
        run: __LINT_CMD__
      - name: Test
        run: __TEST_CMD__
      - name: Build
        run: __BUILD_CMD__
      - uses: actions/upload-artifact@v4
        with:
          name: __APP_NAME__-build
          path: __ARTIFACT_PATH__

  infra:
    name: Infra — validate
    runs-on: ubuntu-latest
    if: ${{ hashFiles('infra/**') != '' }}
    steps:
      - uses: actions/checkout@v4

      - uses: hashicorp/setup-terraform@v3
      - name: Terraform fmt + validate
        if: ${{ hashFiles('infra/terraform/**') != '' }}
        working-directory: infra/terraform
        run: |
          terraform fmt -check -recursive
          terraform init -backend=false
          terraform validate

      - name: Ansible lint
        if: ${{ hashFiles('infra/ansible/**') != '' }}
        run: |
          pip install ansible-core ansible-lint
          cd infra/ansible && ansible-lint site.yml deploy.yml

      - name: Puppet lint
        if: ${{ hashFiles('infra/puppet/**') != '' }}
        run: |
          sudo gem install puppet-lint
          puppet-lint --fail-on-warnings infra/puppet

      - name: Nagios plugin check
        if: ${{ hashFiles('infra/nagios/**') != '' }}
        run: python3 -m py_compile infra/nagios/plugins/check_app.py
```

**Setup step by language** (replace `__SETUP_STEP__`):

| Language | Setup step | Typical commands |
|---|---|---|
| Go | `actions/setup-go@v5` with `go-version-file: go.mod` | `go vet ./...` · `go test ./...` · `CGO_ENABLED=0 go build -o dist/app .` |
| Node.js | `actions/setup-node@v4` with `node-version: 22` and `cache: npm` | `npm ci` · `npm run lint` · `npm test` · `npm run build` |
| Python | `actions/setup-python@v5` with `python-version: "3.12"` | `pip install -r requirements.txt` · `ruff check .` · `pytest` · *(none)* |
| Java | `actions/setup-java@v4` with `distribution: temurin` and `java-version: 21` | `./mvnw -B verify` · `./mvnw -B package` |
| Rust | `dtolnay/rust-toolchain@stable` | `cargo clippy -- -D warnings` · `cargo test` · `cargo build --release` |

### ✅ Verify

- [ ] Push a commit, and the **Actions** tab shows `ci` green.
- [ ] Break a test on purpose, and CI turns red. Revert.
- [ ] *(If chosen)* Branch protection on `main` requires the `ci` checks.

**If skipped:** run the test command by hand before every release. S4 still works.

---

## Stage 4: Release (versioned artifacts)

**Purpose:** pushing a tag like `v1.2.0` produces a downloadable, versioned artifact. Servers always install a specific version, never "whatever is on my laptop".

### 🟡 OWNER CALL S4

- "Publish releases automatically when a `v*` tag is pushed?" *[yes]*
- Artifact type, from S0:
  - **binary**: attach it to a GitHub Release
  - **container**: push the image to GitHub Container Registry (GHCR)
  - **source**: attach a `.tar.gz` of the build
- "Build for extra platforms, such as `linux/arm64` or macOS?" *[linux/amd64 only]*

### Files

**`.github/workflows/release.yml`** (binary or source variant)

```yaml
name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      # __SETUP_STEP__ (same as ci.yml)
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Test
        run: __TEST_CMD__
      - name: Build
        run: __BUILD_CMD__          # must produce __ARTIFACT_PATH__
      - name: Checksums
        run: sha256sum __ARTIFACT_PATH__ > __ARTIFACT_PATH__.sha256
      - name: Publish GitHub Release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh release create "${GITHUB_REF_NAME}" \
            __ARTIFACT_PATH__ __ARTIFACT_PATH__.sha256 \
            --title "${GITHUB_REF_NAME}" --generate-notes
```

**Container variant:** replace the last three steps with these.

```yaml
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: docker/build-push-action@v6
        with:
          push: true
          tags: |
            ghcr.io/${{ github.repository }}:${{ github.ref_name }}
            ghcr.io/${{ github.repository }}:latest
```

For the container variant, also add `packages: write` under `permissions:`.

### Run

```bash
git tag v1.0.0
git push origin v1.0.0
```

### ✅ Verify

- [ ] The release appears under **Releases**, or the image appears under **Packages**.
- [ ] This download works from any machine:

  ```bash
  curl -fL -o /tmp/app https://github.com/__GITHUB_REPO__/releases/download/v1.0.0/__ARTIFACT_NAME__
  ```

**If skipped:** in S6, set `app_install_method: copy` so Ansible copies a locally built artifact to the servers instead of downloading it.

---

## Stage 5: Infrastructure

**Purpose:** create the servers from code, so they can be destroyed and recreated identically.

### 🟡 OWNER CALL S5

1. "Where should the servers come from?"
   - **(a) AWS with Terraform** *[default]*
   - **(b) Local VMs with Vagrant and VirtualBox**. Free and offline, and covers Practical 1 and Unit V.
   - **(c) Existing servers**: you provide the IPs. Skips this stage.
2. If (a):
   - AWS region? *[ap-south-1]*
   - Instance sizes? Defaults: app `t3.micro`, load balancer `t3.micro`, ops `t3.small`. The ops server runs the Puppet server and Nagios, which need about 2 GB of RAM.
   - Your public IP for SSH and web-UI access, as `x.x.x.x/32`?
   - Use **remote state in S3**? *[yes for teams, optional when solo]*
   - Use **workspaces** for `dev` and `prod`? *[yes]*
3. "Roles to create": app × `__APP_COUNT__`, plus a load balancer *(if S0 said yes)*, plus an ops node *(needed if S7 or S8 is included)*.

> ⚠️ **AWS costs money** once you're outside the free tier. Always finish with `terraform destroy` (Stage 11).

### Architecture created

```text
                Internet
                    │ :80
              ┌─────▼─────┐
              │   lb-1    │  nginx load balancer
              └─────┬─────┘
          ┌─────────┴─────────┐   :__APP_PORT__ (VPC only)
     ┌────▼────┐         ┌────▼────┐
     │  app-1  │   ...   │  app-N  │  app + NRPE + Puppet agent
     └────┬────┘         └────┬────┘
          └─────────┬─────────┘   :5666 NRPE, :8140 Puppet (VPC only)
              ┌─────▼─────┐
              │   ops-1   │  Puppet server + Nagios
              └───────────┘   (web UI through an SSH tunnel)
```

### 5A: Terraform on AWS

**`infra/terraform/main.tf`**

```hcl
terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    local = {
      source  = "hashicorp/local"
      version = ">= 2.4"
    }
  }

  # Stage 5.4: uncomment for remote state (create the bucket first)
  # backend "s3" {
  #   bucket       = "__STATE_BUCKET__"
  #   key          = "__APP_NAME__/terraform.tfstate"
  #   region       = "__AWS_REGION__"
  #   use_lockfile = true     # S3-native state locking (Terraform 1.10+)
  #   encrypt      = true
  # }
}

provider "aws" {
  region = var.aws_region
  default_tags {
    tags = {
      Project   = var.project
      Env       = terraform.workspace
      ManagedBy = "terraform"
    }
  }
}

locals {
  env = terraform.workspace # dev, prod, ...

  # Every server, keyed by host name. The "if" filters keep the types consistent
  # (a plain `cond ? {...} : {}` fails because the two objects differ).
  nodes = merge(
    { for k, v in { "lb-1" = { role = "lb", type = var.lb_instance_type } } : k => v if var.enable_lb },
    { for i in range(var.app_count) : "app-${i + 1}" => { role = "app", type = var.app_instance_type } },
    { for k, v in { "ops-1" = { role = "ops", type = var.ops_instance_type } } : k => v if var.enable_ops }
  )
}

module "network" {
  source     = "./modules/network"
  project    = var.project
  env        = local.env
  admin_cidr = var.admin_cidr
  app_port   = var.app_port
}

module "compute" {
  source         = "./modules/compute"
  project        = var.project
  env            = local.env
  nodes          = local.nodes
  subnet_id      = module.network.subnet_id
  sg_by_role     = module.network.sg_by_role
  public_key     = file(pathexpand(var.public_key_path))
  root_volume_gb = 16
}

# Terraform → Ansible hand-off (Practical 15): write the inventory file
resource "local_file" "ansible_inventory" {
  filename        = "${path.module}/../ansible/inventory/hosts.ini"
  file_permission = "0644"
  content = templatefile("${path.module}/inventory.tftpl", {
    nodes    = module.compute.nodes
    ssh_key  = pathexpand(var.private_key_path)
    ssh_user = "ubuntu"
  })
}
```

**`infra/terraform/variables.tf`**

```hcl
variable "project" {
  type    = string
  default = "__APP_NAME__"
}
variable "aws_region" {
  type    = string
  default = "__AWS_REGION__"
}
variable "admin_cidr" {
  description = "Your public IP as x.x.x.x/32. Only this IP can SSH in or open the ops web UIs."
  type        = string
}
variable "app_port" {
  type    = number
  default = __APP_PORT__
}
variable "app_count" {
  type    = number
  default = __APP_COUNT__
}
variable "enable_lb" {
  type    = bool
  default = true
}
variable "enable_ops" {
  type    = bool
  default = true
}
variable "app_instance_type" {
  type    = string
  default = "t3.micro"
}
variable "lb_instance_type" {
  type    = string
  default = "t3.micro"
}
variable "ops_instance_type" {
  type    = string
  default = "t3.small"
}
variable "public_key_path" {
  type    = string
  default = "~/.ssh/id_ed25519.pub"
}
variable "private_key_path" {
  type    = string
  default = "~/.ssh/id_ed25519"
}
```

**`infra/terraform/outputs.tf`**

```hcl
output "nodes" {
  description = "Every server with its role and IP addresses"
  value       = module.compute.nodes
}

output "site_url" {
  description = "Open this in a browser"
  value = try(
    "http://${module.compute.nodes["lb-1"].public_ip}",
    "http://${module.compute.nodes["app-1"].public_ip}:${var.app_port}"
  )
}

output "nagios_tunnel" {
  description = "Run this, then open http://localhost:8090/nagios4"
  value       = try("ssh -L 8090:localhost:80 ubuntu@${module.compute.nodes["ops-1"].public_ip}", "ops node disabled")
}
```

**`infra/terraform/inventory.tftpl`**

```text
# Generated by Terraform. Do not edit by hand.
%{ for role in ["lb", "app", "ops"] ~}
[${role}]
%{ for name, n in nodes ~}
%{ if n.role == role ~}
${name} ansible_host=${n.public_ip} private_ip=${n.private_ip}
%{ endif ~}
%{ endfor ~}

%{ endfor ~}
[all:vars]
ansible_user=${ssh_user}
ansible_ssh_private_key_file=${ssh_key}
ansible_ssh_common_args='-o StrictHostKeyChecking=accept-new'
```

Hosts are named `lb-1`, `app-1`, `app-2` and `ops-1` so that host names never clash with the group names `lb`, `app` and `ops`.

**`infra/terraform/example.tfvars`** (copy it to `terraform.tfvars`, which is git-ignored):

```hcl
admin_cidr = "203.0.113.10/32" # your IP: curl -s https://checkip.amazonaws.com
app_count  = 2
```

**`infra/terraform/modules/network/main.tf`**

```hcl
variable "project" { type = string }
variable "env" { type = string }
variable "admin_cidr" { type = string }
variable "app_port" { type = number }

data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# SSH from the admin's IP only
resource "aws_security_group" "admin" {
  name   = "${var.project}-${var.env}-admin"
  vpc_id = data.aws_vpc.default.id
  ingress {
    description = "SSH from admin"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.admin_cidr]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# Private traffic inside the VPC: app port, Puppet 8140, NRPE 5666
resource "aws_security_group" "internal" {
  name   = "${var.project}-${var.env}-internal"
  vpc_id = data.aws_vpc.default.id
  dynamic "ingress" {
    for_each = toset([var.app_port, 8140, 5666])
    content {
      from_port   = ingress.value
      to_port     = ingress.value
      protocol    = "tcp"
      cidr_blocks = [data.aws_vpc.default.cidr_block]
    }
  }
}

# Public web on the load balancer
resource "aws_security_group" "web" {
  name   = "${var.project}-${var.env}-web"
  vpc_id = data.aws_vpc.default.id
  dynamic "ingress" {
    for_each = toset([80, 443])
    content {
      from_port   = ingress.value
      to_port     = ingress.value
      protocol    = "tcp"
      cidr_blocks = ["0.0.0.0/0"]
    }
  }
}

output "subnet_id" {
  value = sort(data.aws_subnets.default.ids)[0]
}

output "sg_by_role" {
  value = {
    lb  = [aws_security_group.admin.id, aws_security_group.web.id, aws_security_group.internal.id]
    app = [aws_security_group.admin.id, aws_security_group.internal.id]
    ops = [aws_security_group.admin.id, aws_security_group.internal.id]
  }
}
```

**`infra/terraform/modules/compute/main.tf`**

```hcl
variable "project" { type = string }
variable "env" { type = string }
variable "nodes" {
  type = map(object({ role = string, type = string }))
}
variable "subnet_id" { type = string }
variable "sg_by_role" { type = map(list(string)) }
variable "public_key" { type = string }
variable "root_volume_gb" {
  type    = number
  default = 16
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }
}

resource "aws_key_pair" "this" {
  key_name   = "${var.project}-${var.env}"
  public_key = var.public_key
}

resource "aws_instance" "this" {
  for_each               = var.nodes
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = each.value.type
  subnet_id              = var.subnet_id
  key_name               = aws_key_pair.this.key_name
  vpc_security_group_ids = var.sg_by_role[each.value.role]

  associate_public_ip_address = true

  root_block_device {
    volume_size = var.root_volume_gb
    volume_type = "gp3"
  }

  tags = {
    Name = "${var.project}-${var.env}-${each.key}"
    Role = each.value.role
  }
}

output "nodes" {
  value = {
    for name, i in aws_instance.this : name => {
      role       = var.nodes[name].role
      public_ip  = i.public_ip
      private_ip = i.private_ip
    }
  }
}
```

#### Run

```bash
cd infra/terraform
cp example.tfvars terraform.tfvars      # then edit admin_cidr
terraform init
terraform workspace new dev             # or: terraform workspace select dev
terraform fmt -recursive
terraform validate
terraform plan -out tfplan
terraform apply tfplan
terraform output
```

#### 5.4: Remote state (optional, owner's call)

1. Create an S3 bucket once, by hand or in a separate tiny Terraform config. Turn on **versioning** and **block public access**.
2. Uncomment the `backend "s3"` block and run `terraform init -migrate-state`.
3. Explain in the presentation: the state lives in one shared, locked place (`use_lockfile`), so two people can't apply at the same time.

#### ✅ Verify

- [ ] `terraform plan` immediately after `apply` reports **No changes**. This is idempotency.
- [ ] `infra/ansible/inventory/hosts.ini` exists and lists every node.
- [ ] `ssh -i ~/.ssh/id_ed25519 ubuntu@<app-1 ip>` works.
- [ ] `terraform state list` shows the instances, security groups and key pair.

### 5B: Vagrant (local VMs)

**`Vagrantfile`**

```ruby
# Local copy of the AWS layout. Needs VirtualBox and Vagrant.
NODES = {
  "lb-1"  => { ip: "192.168.56.10", mem: 512  },
  "app-1" => { ip: "192.168.56.11", mem: 1024 },
  "app-2" => { ip: "192.168.56.12", mem: 1024 },
  "ops-1" => { ip: "192.168.56.20", mem: 3072 },  # Puppet server + Nagios
}

Vagrant.configure("2") do |config|
  config.vm.box = "bento/ubuntu-24.04"
  NODES.each do |name, opts|
    config.vm.define name do |node|
      node.vm.hostname = name
      node.vm.network "private_network", ip: opts[:ip]
      node.vm.provider "virtualbox" do |vb|
        vb.memory = opts[:mem]
        vb.cpus   = 1
      end
    end
  end
end
```

**`infra/ansible/inventory/hosts.ini`** for Vagrant (written by hand, because there's no Terraform):

```ini
[lb]
lb-1  ansible_host=192.168.56.10 private_ip=192.168.56.10
[app]
app-1 ansible_host=192.168.56.11 private_ip=192.168.56.11
app-2 ansible_host=192.168.56.12 private_ip=192.168.56.12
[ops]
ops-1 ansible_host=192.168.56.20 private_ip=192.168.56.20

[all:vars]
ansible_user=vagrant
ansible_ssh_common_args='-o StrictHostKeyChecking=no'
```

Get each VM's SSH key path from `vagrant ssh-config` and set it per host with `ansible_ssh_private_key_file`.

#### Run and verify

```bash
vagrant up
ansible all -m ping          # every host replies "pong"
```

### 5C: Existing servers

Write `infra/ansible/inventory/hosts.ini` by hand in the same format as 5B, using your servers' IPs and SSH user.

**If skipped entirely:** S6–S8 need some inventory to work with, so use 5B or 5C. If the owner wants **no servers at all**, skip S6–S8 and finish at S4.

---

## Stage 6: Configuration and deployment (Ansible)

**Purpose:** turn bare servers into a working system with one command, then roll out new versions with zero downtime. Ansible **pushes** changes when you run it.

### 🟡 OWNER CALL S6

1. "Include Ansible?" *[yes]*
2. "Install method for the app?"
   - **release**: download from GitHub Releases or GHCR *[default if S4 is included]*
   - **copy**: copy a locally built artifact
3. "Roll out **one server at a time** (rolling deploy)?" *[yes if there are 2+ app servers]*
4. "Ask for the version at deploy time (a prompt), or always deploy the latest tag?" *[prompt]*
5. "Who owns ongoing configuration files such as the nginx config and the cleanup cron?"
   - **puppet** if S7 is included
   - **ansible** otherwise

   This prevents the two tools fighting over the same files.

### Files

**`infra/ansible/ansible.cfg`**

```ini
[defaults]
inventory = inventory/hosts.ini
roles_path = roles
host_key_checking = False
retry_files_enabled = False
interpreter_python = auto_silent
callback_result_format = yaml

[ssh_connection]
pipelining = True
```

**`infra/ansible/group_vars/all.yml`**

```yaml
app_name: __APP_NAME__
app_repo: __GITHUB_REPO__
app_port: __APP_PORT__
app_health_path: __HEALTH_PATH__
app_artifact: __ARTIFACT_TYPE__            # binary | container | source
app_artifact_name: __ARTIFACT_NAME__
app_install_method: release                # release | copy
app_local_artifact: "{{ playbook_dir }}/../../dist/__ARTIFACT_NAME__"   # used when method = copy
app_run_args: "__RUN_ARGS__"               # arguments after the binary, e.g. "api --port 8000"
app_env:                                   # environment variables for the app
  PORT: "{{ app_port }}"
app_system_packages: [__SYSTEM_PACKAGES__]

config_owner: puppet                       # puppet | ansible
puppet_enabled: true
nagios_enabled: true
ops_private_ip: "{{ hostvars[groups['ops'][0]]['private_ip'] if groups.get('ops') else '' }}"
```

**`infra/ansible/site.yml`**: builds everything from scratch.

```yaml
- name: Baseline for every server
  hosts: all
  become: true
  roles:
    - common

- name: Ops server (Puppet server, Nagios)
  hosts: ops
  become: true
  roles:
    - role: puppet_server
      when: puppet_enabled
    - role: nagios
      when: nagios_enabled

- name: App servers
  hosts: app
  become: true
  roles:
    - app
    - role: nrpe
      when: nagios_enabled
    - role: puppet_agent
      when: puppet_enabled

- name: Load balancer
  hosts: lb
  become: true
  roles:
    - role: lb
      when: config_owner == 'ansible'
    - role: puppet_agent
      when: puppet_enabled
```

**`infra/ansible/deploy.yml`**: rolling release of a new version.

```yaml
- name: Rolling deploy
  hosts: app
  become: true
  serial: 1                     # one server at a time, so the site stays up
  vars_prompt:
    - name: app_version
      prompt: "Version tag to deploy (e.g. v1.2.0)"
      private: false
  pre_tasks:
    - name: Silence Nagios alerts for this host while it updates
      community.general.nagios:
        action: downtime
        host: "{{ inventory_hostname }}"
        services: host
        minutes: 10
        cmdfile: /var/lib/nagios4/rw/nagios.cmd
        comment: "Ansible rolling deploy {{ app_version }}"
      delegate_to: "{{ groups['ops'][0] }}"
      when: nagios_enabled
      tags: [nagios]
  roles:
    - app
  post_tasks:
    - name: Wait until the app is healthy before moving to the next server
      ansible.builtin.uri:
        url: "http://127.0.0.1:{{ app_port }}{{ app_health_path }}"
        status_code: 200
      register: health
      retries: 10
      delay: 3
      until: health.status == 200
```

**`roles/common/tasks/main.yml`**

```yaml
- name: Update apt cache and install basics
  ansible.builtin.apt:
    name: [curl, ca-certificates, htop, ufw, unattended-upgrades]
    state: present
    update_cache: true
    cache_valid_time: 3600

- name: Set hostname to the inventory name
  ansible.builtin.hostname:
    name: "{{ inventory_hostname }}"

- name: Let every node find the Puppet server as "puppet"
  ansible.builtin.lineinfile:
    path: /etc/hosts
    line: "{{ ops_private_ip }} puppet ops"
    regexp: '\spuppet(\s|$)'
  when: puppet_enabled and ops_private_ip | length > 0

- name: Record this node's role (read by a custom Puppet fact)
  ansible.builtin.copy:
    dest: /etc/node_role
    content: "{{ group_names | first }}\n"
    mode: "0644"
```

**`roles/app/tasks/main.yml`** (binary artifact; see the notes below for other artifact types)

```yaml
- name: Install packages the app needs
  ansible.builtin.apt:
    name: "{{ app_system_packages }}"
    state: present
  when: app_system_packages | length > 0

- name: Create a system user for the app
  ansible.builtin.user:
    name: "{{ app_name }}"
    system: true
    shell: /usr/sbin/nologin
    create_home: false

- name: Create app folders
  ansible.builtin.file:
    path: "{{ item }}"
    state: directory
    owner: "{{ app_name }}"
    mode: "0755"
  loop:
    - "/opt/{{ app_name }}/releases"
    - "/var/lib/{{ app_name }}"

- name: Get the release
  block:
    - name: Download from GitHub Releases
      ansible.builtin.get_url:
        url: "https://github.com/{{ app_repo }}/releases/download/{{ app_version }}/{{ app_artifact_name }}"
        dest: "/opt/{{ app_name }}/releases/{{ app_name }}-{{ app_version }}"
        mode: "0755"
      when: app_install_method == 'release'

    - name: Copy a locally built artifact
      ansible.builtin.copy:
        src: "{{ app_local_artifact }}"
        dest: "/opt/{{ app_name }}/releases/{{ app_name }}-{{ app_version }}"
        mode: "0755"
      when: app_install_method == 'copy'

    - name: Point "current" at this version
      ansible.builtin.file:
        src: "/opt/{{ app_name }}/releases/{{ app_name }}-{{ app_version }}"
        dest: "/opt/{{ app_name }}/current"
        state: link
        force: true
      notify: Restart app
  rescue:
    - name: Explain the failure
      ansible.builtin.fail:
        msg: "Could not install {{ app_name }} {{ app_version }}. Does that release exist?"

- name: Install the systemd service
  ansible.builtin.template:
    src: app.service.j2
    dest: "/etc/systemd/system/{{ app_name }}.service"
    mode: "0644"
  notify:
    - Reload systemd
    - Restart app

- name: Start the app now and at boot
  ansible.builtin.systemd_service:
    name: "{{ app_name }}"
    state: started
    enabled: true

- name: Apply pending restarts before checking health
  ansible.builtin.meta: flush_handlers
```

**`roles/app/defaults/main.yml`**

```yaml
app_version: "latest-known"   # override with -e app_version=v1.2.0 or the deploy.yml prompt
```

> When running `site.yml`, pass a version: `ansible-playbook site.yml -e app_version=v1.0.0`.

**`roles/app/handlers/main.yml`**

```yaml
- name: Reload systemd
  ansible.builtin.systemd_service:
    daemon_reload: true

- name: Restart app
  ansible.builtin.systemd_service:
    name: "{{ app_name }}"
    state: restarted
```

**`roles/app/templates/app.service.j2`**

```ini
[Unit]
Description={{ app_name }} (managed by Ansible)
After=network-online.target
Wants=network-online.target

[Service]
User={{ app_name }}
WorkingDirectory=/var/lib/{{ app_name }}
ExecStart=/opt/{{ app_name }}/current {{ app_run_args }}
{% for k, v in app_env.items() %}
Environment={{ k }}={{ v }}
{% endfor %}
Restart=on-failure
RestartSec=2
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/{{ app_name }} /tmp

[Install]
WantedBy=multi-user.target
```

**Other artifact types:**

- **container**: install `docker.io`, then use `community.docker.docker_container` with `image: ghcr.io/{{ app_repo }}:{{ app_version }}`, `published_ports: ["{{ app_port }}:{{ app_port }}"]` and `restart_policy: unless-stopped`. Skip the systemd template.
- **source**: install the runtime (`nodejs` or `python3-venv`), unpack the release `.tar.gz` into the release folder, install dependencies (`npm ci --omit=dev` or `pip install -r requirements.txt` in a venv), and keep the systemd service with `ExecStart` pointing at the runtime.

**`roles/lb/tasks/main.yml`**: used only when `config_owner: ansible`. Otherwise Puppet manages nginx (S7).

```yaml
- name: Install nginx
  ansible.builtin.apt:
    name: nginx
    state: present

- name: Load-balancer config
  ansible.builtin.template:
    src: lb.conf.j2
    dest: /etc/nginx/sites-available/default
    mode: "0644"
    validate: "nginx -t -c /etc/nginx/nginx.conf"
  notify: Reload nginx

- name: Start nginx
  ansible.builtin.systemd_service:
    name: nginx
    state: started
    enabled: true
```

**`roles/lb/handlers/main.yml`**

```yaml
- name: Reload nginx
  ansible.builtin.systemd_service:
    name: nginx
    state: reloaded
```

**`roles/lb/templates/lb.conf.j2`**

```nginx
upstream {{ app_name }}_backend {
{% for h in groups['app'] %}
    server {{ hostvars[h].private_ip }}:{{ app_port }} max_fails=2 fail_timeout=10s;
{% endfor %}
}

server {
    listen 80 default_server;
    client_max_body_size 25m;

    location / {
        proxy_pass http://{{ app_name }}_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_next_upstream error timeout http_502 http_503;
    }
}
```

> The `validate:` line checks the new file with `nginx -t` before replacing the old one. Note that `nginx -t -c /etc/nginx/nginx.conf` tests the *installed* config rather than the temporary file. For strict validation, render to a temp path and run `nginx -t` on a config that includes it.

The roles `nrpe`, `nagios`, `puppet_server` and `puppet_agent` are listed in S7 and S8.

### Run

```bash
cd infra/ansible
ansible all -m ping                                   # ad-hoc command (Practical 11)
ansible-playbook site.yml -e app_version=v1.0.0       # build everything
ansible-playbook site.yml -e app_version=v1.0.0 --check --diff   # dry run
ansible-playbook deploy.yml                           # later releases: asks for the version
ansible-playbook site.yml --tags nagios               # run only the tagged parts
```

Collections needed: `ansible-galaxy collection install community.general ansible.posix community.docker`

### ✅ Verify

- [ ] `ansible-playbook site.yml …` finishes with `failed=0`.
- [ ] Running it a **second time** shows `changed=0` on most tasks. This is idempotency.
- [ ] `curl http://<lb ip>/` serves the app, and refreshing spreads requests across app servers (check each app's logs).
- [ ] During `deploy.yml`, the site keeps working (run `while true; do curl -s -o /dev/null -w "%{http_code}\n" http://<lb>/; sleep 1; done`).

**If skipped:** you'll have to install the app by hand. S7 can install the app itself instead: give the Puppet module a `package` or `file` resource for the artifact, and an `exec` to download it.

---

## Stage 7: State enforcement (Puppet / OpenVox)

**Purpose:** keep every server in its declared state **all the time**. Each agent **pulls** its configuration from the Puppet server every 30 minutes and fixes any drift, such as an edited config, a stopped service or a deleted file.

> **Puppet vs OpenVox:** OpenVox is the community-maintained open-source continuation of Puppet. It uses the same language, commands (`puppet`, `puppetserver`) and paths (`/etc/puppetlabs/`). This template installs OpenVox 8 from `apt.voxpupuli.org`. Everything you learn from Puppet books applies.

### 🟡 OWNER CALL S7

1. "Include Puppet?" *[yes]*
2. "Who owns which files?" This template's default, which you should explain in the presentation:

   | Task | Ansible (push, at deploy time) | Puppet (pull, every 30 min) |
   |---|---|---|
   | Install packages, OpenVox, Nagios | ✅ | |
   | Deploy a new app version | ✅ | |
   | nginx load-balancer config | | ✅ (template + facts) |
   | Keep the app service running | | ✅ |
   | Cleanup cron for temp files | | ✅ |
   | Message of the day / banner | | ✅ |

3. "Sign agent certificates **by hand**, to show the certificate exchange in Practical 5? Or turn on **autosign** for the lab?" *[by hand]*
4. "Agent run interval?" *[30 minutes. For a demo, trigger runs with `puppet agent -t`.]*

### Files: Ansible roles that install OpenVox

**`roles/puppet_server/tasks/main.yml`**

```yaml
- name: Add the OpenVox 8 apt repository
  ansible.builtin.apt:
    deb: "https://apt.voxpupuli.org/openvox8-release-ubuntu{{ ansible_distribution_version }}.deb"

- name: Install OpenVox server
  ansible.builtin.apt:
    name: openvox-server
    state: present
    update_cache: true

- name: Use less memory on a small VM (default heap is 2 GB)
  ansible.builtin.lineinfile:
    path: /etc/default/puppetserver
    regexp: '^JAVA_ARGS='
    line: 'JAVA_ARGS="-Xms512m -Xmx1g"'
  notify: Restart puppetserver

- name: Certificate names the server answers to
  community.general.ini_file:
    path: /etc/puppetlabs/puppet/puppet.conf
    section: server
    option: dns_alt_names
    value: "puppet,ops,{{ inventory_hostname }}"
    mode: "0644"

- name: Ship our Puppet code to the server
  ansible.posix.synchronize:
    src: "{{ playbook_dir }}/../puppet/"
    dest: /etc/puppetlabs/code/environments/production/
    delete: false
  tags: [puppet_code]

- name: Tell Puppet which servers are app servers (Hiera data)
  ansible.builtin.copy:
    dest: /etc/puppetlabs/code/environments/production/data/nodes.yaml
    content: |
      app_backends:
      {% for h in groups['app'] %}
        - "{{ hostvars[h].private_ip }}"
      {% endfor %}
    mode: "0644"
  tags: [puppet_code]

- name: Start puppetserver
  ansible.builtin.systemd_service:
    name: puppetserver
    state: started
    enabled: true
```

**`roles/puppet_server/handlers/main.yml`**

```yaml
- name: Restart puppetserver
  ansible.builtin.systemd_service:
    name: puppetserver
    state: restarted
```

**`roles/puppet_agent/tasks/main.yml`**

```yaml
- name: Add the OpenVox 8 apt repository
  ansible.builtin.apt:
    deb: "https://apt.voxpupuli.org/openvox8-release-ubuntu{{ ansible_distribution_version }}.deb"

- name: Install OpenVox agent
  ansible.builtin.apt:
    name: openvox-agent
    state: present
    update_cache: true

- name: Point the agent at the Puppet server
  community.general.ini_file:
    path: /etc/puppetlabs/puppet/puppet.conf
    section: agent
    option: "{{ item.k }}"
    value: "{{ item.v }}"
    mode: "0644"
  loop:
    - { k: server, v: puppet }
    - { k: runinterval, v: 30m }

- name: Start the agent service
  ansible.builtin.systemd_service:
    name: puppet
    state: started
    enabled: true
```

> `ansible.posix.synchronize` needs `rsync` on both machines and the `ansible.posix` collection: `ansible-galaxy collection install ansible.posix`.

### Files: Puppet code (`infra/puppet/`)

**`hiera.yaml`**

```yaml
version: 5
defaults:
  datadir: data
  data_hash: yaml_data
hierarchy:
  - name: "Generated node data"
    path: "nodes.yaml"
  - name: "Common"
    path: "common.yaml"
```

**`data/common.yaml`**

```yaml
__APP_NAME__::app_name: "__APP_NAME__"
__APP_NAME__::app_port: __APP_PORT__
__APP_NAME__::cleanup_minutes: 60
```

**`manifests/site.pp`**: assigns classes to nodes by role.

```puppet
# Classify nodes by the custom fact `node_role` (written by Ansible to /etc/node_role)
node default {
  include __APP_NAME__::base

  case $facts['node_role'] {
    'app':   { include __APP_NAME__::app }
    'lb':    { include __APP_NAME__::lb }
    default: { }
  }
}
```

**`modules/__APP_NAME__/manifests/base.pp`**

```puppet
# @summary Settings every managed node gets.
class __APP_NAME__::base {
  file { '/etc/motd':
    ensure  => file,
    content => "This server is managed by Puppet. Role: ${facts['node_role']}. Manual changes will be reverted.\n",
  }
}
```

**`modules/__APP_NAME__/manifests/app.pp`**

```puppet
# @summary Keep the app service healthy and its temp files cleaned.
class __APP_NAME__::app (
  String  $app_name        = lookup('__APP_NAME__::app_name'),
  Integer $cleanup_minutes = lookup('__APP_NAME__::cleanup_minutes'),
) {
  service { $app_name:
    ensure => running,
    enable => true,
  }

  cron { "${app_name}-cleanup":
    command => "find /var/lib/${app_name} -type f -mmin +${cleanup_minutes} -delete",
    user    => 'root',
    minute  => '*/15',
  }
}
```

**`modules/__APP_NAME__/manifests/lb.pp`**

```puppet
# @summary nginx load balancer built from facts and Hiera data.
class __APP_NAME__::lb (
  String        $app_name     = lookup('__APP_NAME__::app_name'),
  Integer       $app_port     = lookup('__APP_NAME__::app_port'),
  Array[String] $app_backends = lookup('app_backends'),
) {
  # custom function (functions/backend_list.pp) → ['10.0.0.5:8000', ...]
  $backends = __APP_NAME__::backend_list($app_backends, $app_port)

  package { 'nginx':
    ensure => installed,
  }

  file { '/etc/nginx/sites-available/default':
    ensure  => file,
    content => template('__APP_NAME__/lb.conf.erb'),
    require => Package['nginx'],
    notify  => Service['nginx'],
  }

  service { 'nginx':
    ensure => running,
    enable => true,
  }
}
```

**`modules/__APP_NAME__/templates/lb.conf.erb`**

```erb
# Managed by Puppet on <%= scope['facts']['networking']['fqdn'] %>. Edits will be reverted.
upstream <%= @app_name %>_backend {
<% @backends.each do |b| -%>
    server <%= b %> max_fails=2 fail_timeout=10s;
<% end -%>
}

server {
    listen 80 default_server;
    client_max_body_size 25m;
    location / {
        proxy_pass http://<%= @app_name %>_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_next_upstream error timeout http_502 http_503;
    }
}
```

**`modules/__APP_NAME__/functions/backend_list.pp`**: a custom function written in the Puppet language.

```puppet
# @summary Turn ['10.0.0.5', '10.0.0.6'] and 8000 into ['10.0.0.5:8000', '10.0.0.6:8000'].
function __APP_NAME__::backend_list(Array[String] $ips, Integer $port) >> Array[String] {
  $ips.map |$ip| { "${ip}:${port}" }
}
```

**`modules/__APP_NAME__/lib/facter/node_role.rb`**: a custom fact.

```ruby
# Reads the role Ansible wrote to /etc/node_role (app, lb or ops).
Facter.add(:node_role) do
  setcode do
    File.exist?('/etc/node_role') ? File.read('/etc/node_role').strip : 'unknown'
  end
end
```

**`modules/__APP_NAME__/lib/facter/app_version.rb`**: a second custom fact.

```ruby
# Which app version is deployed (from the "current" symlink target).
Facter.add(:app_version) do
  setcode do
    link = '/opt/__APP_NAME__/current'
    File.symlink?(link) ? File.basename(File.readlink(link)).sub(/^__APP_NAME__-/, '') : nil
  end
end
```

### Run

```bash
# 1. Install the server and agents
ansible-playbook site.yml -e app_version=v1.0.0

# 2. Certificate exchange (Practical 5)
ssh ubuntu@<app-1> 'sudo /opt/puppetlabs/bin/puppet agent -t'   # sends a cert request
ssh ubuntu@<ops>   'sudo /opt/puppetlabs/bin/puppetserver ca list'
ssh ubuntu@<ops>   'sudo /opt/puppetlabs/bin/puppetserver ca sign --all'
ssh ubuntu@<app-1> 'sudo /opt/puppetlabs/bin/puppet agent -t'   # applies the catalog

# Practise a manifest on one node without the server (Puppet in isolation)
sudo /opt/puppetlabs/bin/puppet apply -e "file { '/tmp/hello': content => 'hi' }"

# After changing Puppet code, ship it again
ansible-playbook site.yml --tags puppet_code
```

### ✅ Verify

- [ ] `puppetserver ca list --all` shows every agent as signed.
- [ ] `puppet agent -t` on each node ends without errors.
- [ ] `facter -p node_role app_version` prints the right values on an app node.
- [ ] **The drift test:** `sudo systemctl stop __APP_NAME__`, then `sudo puppet agent -t`. The service is running again.
- [ ] **The drift test on the LB:** edit `/etc/nginx/sites-available/default`, then run `puppet agent -t`. The file is restored and nginx reloads.

**If skipped:** set `config_owner: ansible` and `puppet_enabled: false`. The `lb` Ansible role then manages nginx, and drift is only fixed when you rerun Ansible.

---

## Stage 8: Monitoring (Nagios)

**Purpose:** know the moment something breaks: a host is down, the app stops answering, a disk is full, or the core feature stopped working.

### 🟡 OWNER CALL S8

1. "Include Nagios?" *[yes]*
2. "Install from Ubuntu packages (`nagios4`) or compile from source?" *[packages. Compiling is a good extra for Practical 7 if there's time.]*
3. "Add a **custom plugin** that tests the app's real function, `__FUNCTIONAL_CHECK__`, not just its health page?" *[yes]*
4. "How to reach the web UI?"
   - **SSH tunnel** *[default, and the safest]*
   - Open port 80 on the ops server to your IP only
5. "Send email alerts?" *[no for the lab]*

### What gets monitored

| Host | Service | How | Warning / Critical |
|---|---|---|---|
| all | Host up | ping (`check-host-alive`) | — |
| all | SSH | `check_ssh` | — |
| app-* | Disk `/` | NRPE `check_disk` | 20% / 10% free |
| app-* | Load | NRPE `check_load` | 2 / 4 |
| app-* | App process | NRPE `check_procs -C __APP_NAME__` | 0 processes is critical |
| app-* | Health page | `check_http -p __APP_PORT__ -u __HEALTH_PATH__` | 1s / 3s |
| app-* | **Real function** | custom `check_app.py` | 2s / 5s |
| lb | Website | `check_http -H <lb>` | 1s / 3s |
| ops | Puppet server | `check_tcp -p 8140` | — |

### Files

**`infra/nagios/plugins/check_app.py`**: a custom plugin (Practical 8). It uses only the Python standard library.

```python
#!/usr/bin/env python3
"""Nagios plugin: check that the app is up AND that its real feature works.

Exit codes follow the Nagios plugin rules: 0 OK, 1 WARNING, 2 CRITICAL, 3 UNKNOWN.

Examples:
  check_app.py -H 10.0.1.5 -p 8000 --path /healthz
  check_app.py -H 10.0.1.5 -p 8000 --path /api/compress --upload /usr/local/share/sample.jpg \
               --field file --expect-type image/
"""
import argparse
import mimetypes
import os
import sys
import time
import urllib.error
import urllib.request
import uuid

OK, WARNING, CRITICAL, UNKNOWN = 0, 1, 2, 3
LABELS = {OK: "OK", WARNING: "WARNING", CRITICAL: "CRITICAL", UNKNOWN: "UNKNOWN"}


def finish(code, message, seconds=None, size=None):
    perf = []
    if seconds is not None:
        perf.append(f"time={seconds:.3f}s")
    if size is not None:
        perf.append(f"size={size}B")
    perf_text = (" | " + " ".join(perf)) if perf else ""
    print(f"APP {LABELS[code]} - {message}{perf_text}")
    sys.exit(code)


def multipart(field, path):
    boundary = uuid.uuid4().hex
    filename = os.path.basename(path)
    ctype = mimetypes.guess_type(filename)[0] or "application/octet-stream"
    with open(path, "rb") as f:
        data = f.read()
    body = (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="{field}"; filename="{filename}"\r\n'
        f"Content-Type: {ctype}\r\n\r\n"
    ).encode() + data + f"\r\n--{boundary}--\r\n".encode()
    return body, f"multipart/form-data; boundary={boundary}"


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("-H", "--host", required=True)
    ap.add_argument("-p", "--port", type=int, default=80)
    ap.add_argument("--path", default="/healthz")
    ap.add_argument("--upload", help="file to POST as multipart form data")
    ap.add_argument("--field", default="file", help="form field name for --upload")
    ap.add_argument("--expect-text", help="text that must appear in the response body")
    ap.add_argument("--expect-type", help="Content-Type prefix the response must have")
    ap.add_argument("-w", "--warning", type=float, default=2.0, help="seconds")
    ap.add_argument("-c", "--critical", type=float, default=5.0, help="seconds")
    ap.add_argument("-t", "--timeout", type=float, default=15.0)
    args = ap.parse_args()

    url = f"http://{args.host}:{args.port}{args.path}"
    headers = {"User-Agent": "nagios-check_app"}
    data = None
    if args.upload:
        if not os.path.isfile(args.upload):
            finish(UNKNOWN, f"sample file not found: {args.upload}")
        data, headers["Content-Type"] = multipart(args.field, args.upload)

    start = time.monotonic()
    try:
        req = urllib.request.Request(url, data=data, headers=headers,
                                     method="POST" if data else "GET")
        with urllib.request.urlopen(req, timeout=args.timeout) as res:
            body = res.read()
            status = res.status
            ctype = res.headers.get("Content-Type", "")
    except urllib.error.HTTPError as e:
        finish(CRITICAL, f"{url} returned HTTP {e.code}", time.monotonic() - start)
    except Exception as e:  # connection refused, timeout, DNS ...
        finish(CRITICAL, f"{url} unreachable: {e}")
    elapsed = time.monotonic() - start

    if status != 200:
        finish(CRITICAL, f"{url} returned HTTP {status}", elapsed, len(body))
    if args.expect_type and not ctype.startswith(args.expect_type):
        finish(CRITICAL, f"unexpected Content-Type {ctype!r}", elapsed, len(body))
    if args.expect_text and args.expect_text.encode() not in body:
        finish(CRITICAL, f"response missing {args.expect_text!r}", elapsed, len(body))
    if elapsed >= args.critical:
        finish(CRITICAL, f"slow response {elapsed:.2f}s", elapsed, len(body))
    if elapsed >= args.warning:
        finish(WARNING, f"slow response {elapsed:.2f}s", elapsed, len(body))
    finish(OK, f"HTTP 200 in {elapsed:.2f}s", elapsed, len(body))


if __name__ == "__main__":
    main()
```

**`roles/nagios/tasks/main.yml`** (runs on ops)

```yaml
- name: Install Nagios, plugins and the NRPE client
  ansible.builtin.apt:
    name: [nagios4, monitoring-plugins, nagios-nrpe-plugin, apache2]
    state: present

- name: Allow external commands (Ansible uses this to schedule downtime)
  ansible.builtin.lineinfile:
    path: /etc/nagios4/nagios.cfg
    regexp: '^check_external_commands='
    line: check_external_commands=1
  notify: Restart nagios

- name: Install the custom plugin
  ansible.builtin.copy:
    src: "{{ playbook_dir }}/../nagios/plugins/check_app.py"
    dest: /usr/local/lib/nagios/plugins/check_app.py
    mode: "0755"

- name: Sample file for the functional check
  ansible.builtin.copy:
    src: "{{ nagios_sample_file }}"
    dest: /usr/local/share/nagios-sample{{ nagios_sample_file | splitext | last }}
    mode: "0644"
  when: nagios_sample_file is defined

- name: Hosts, services and commands for this project
  ansible.builtin.template:
    src: project.cfg.j2
    dest: /etc/nagios4/conf.d/{{ app_name }}.cfg
    mode: "0644"
    validate: "/usr/sbin/nagios4 -v /etc/nagios4/nagios.cfg"
  notify: Restart nagios

- name: Start Nagios
  ansible.builtin.systemd_service:
    name: nagios4
    state: started
    enabled: true
```

> The `validate:` line here checks the main config, which includes `conf.d`. If validation fails, run `sudo nagios4 -v /etc/nagios4/nagios.cfg` on the ops server to see the exact error.

**`roles/nagios/handlers/main.yml`**

```yaml
- name: Restart nagios
  ansible.builtin.systemd_service:
    name: nagios4
    state: restarted
```

**`roles/nagios/templates/project.cfg.j2`**

```cfg
# Managed by Ansible: {{ app_name }} monitoring

define command {
    command_name    check_app
    command_line    /usr/local/lib/nagios/plugins/check_app.py -H $HOSTADDRESS$ $ARG1$
}

{% for h in groups['all'] %}
define host {
    use                 linux-server
    host_name           {{ h }}
    address             {{ hostvars[h].private_ip }}
    hostgroups          {{ hostvars[h].group_names | first }}-servers
}
{% endfor %}

define hostgroup {
    hostgroup_name  app-servers
    alias           App servers
}
define hostgroup {
    hostgroup_name  lb-servers
    alias           Load balancers
}
define hostgroup {
    hostgroup_name  ops-servers
    alias           Ops servers
}

define service {
    use                     generic-service
    hostgroup_name          app-servers,lb-servers,ops-servers
    service_description     SSH
    check_command           check_ssh
}

define service {
    use                     generic-service
    hostgroup_name          app-servers
    service_description     Disk /
    check_command           check_nrpe!check_root_disk
}

define service {
    use                     generic-service
    hostgroup_name          app-servers
    service_description     Load
    check_command           check_nrpe!check_load
}

define service {
    use                     generic-service
    hostgroup_name          app-servers
    service_description     {{ app_name }} process
    check_command           check_nrpe!check_app_proc
}

define service {
    use                     generic-service
    hostgroup_name          app-servers
    service_description     {{ app_name }} health
    check_command           check_app!-p {{ app_port }} --path {{ app_health_path }} -w 1 -c 3
}

{% if nagios_functional_args is defined %}
define service {
    use                     generic-service
    hostgroup_name          app-servers
    service_description     {{ app_name }} real function
    check_command           check_app!-p {{ app_port }} {{ nagios_functional_args }}
    max_check_attempts      3
}
{% endif %}

define service {
    use                     generic-service
    hostgroup_name          lb-servers
    service_description     Website via LB
    check_command           check_http
}

define service {
    use                     generic-service
    hostgroup_name          ops-servers
    service_description     Puppet server
    check_command           check_tcp!8140
}
```

Set the functional check in `group_vars/all.yml` when S0 defined one. For example:

```yaml
nagios_sample_file: "{{ playbook_dir }}/files/sample.jpg"   # put a small sample input here
nagios_functional_args: "--path /api/compress --upload /usr/local/share/nagios-sample.jpg --expect-type image/ -w 2 -c 5"
```

**`roles/nrpe/tasks/main.yml`** (runs on app servers)

```yaml
- name: Install the NRPE agent and plugins
  ansible.builtin.apt:
    name: [nagios-nrpe-server, monitoring-plugins]
    state: present

- name: Allow only the ops server to ask for checks
  ansible.builtin.lineinfile:
    path: /etc/nagios/nrpe.cfg
    regexp: '^allowed_hosts='
    line: "allowed_hosts=127.0.0.1,{{ ops_private_ip }}"
  notify: Restart nrpe

- name: Project checks
  ansible.builtin.copy:
    dest: /etc/nagios/nrpe.d/{{ app_name }}.cfg
    mode: "0644"
    content: |
      command[check_root_disk]=/usr/lib/nagios/plugins/check_disk -w 20% -c 10% -p /
      command[check_load]=/usr/lib/nagios/plugins/check_load -w 2,2,2 -c 4,4,4
      command[check_app_proc]=/usr/lib/nagios/plugins/check_procs -c 1: -a {{ app_name }}
  notify: Restart nrpe
```

**`roles/nrpe/handlers/main.yml`**

```yaml
- name: Restart nrpe
  ansible.builtin.systemd_service:
    name: nagios-nrpe-server
    state: restarted
```

### Run

```bash
ansible-playbook site.yml -e app_version=v1.0.0 --tags nagios   # or the whole site.yml

# Open the web UI through a tunnel (nothing exposed to the internet)
ssh -L 8090:localhost:80 ubuntu@<ops public ip>
# then browse to http://localhost:8090/nagios4
```

Ubuntu's `nagios4` package only lets local and private addresses open the web UI, which is why the tunnel is used. Login is **off** by default (`use_authentication=0` in `/etc/nagios4/cgi.cfg`), so anyone who reaches the page can run commands. That's acceptable behind the tunnel. To turn login on, create a user, then set `use_authentication=1` and switch `Require all granted` to `Require valid-user` in `/etc/apache2/conf-available/nagios4-cgi.conf`:

```bash
sudo htdigest -c /etc/nagios4/htdigest.users Nagios4 nagiosadmin
```

Test the plugin by hand:

```bash
/usr/local/lib/nagios/plugins/check_app.py -H <app private ip> -p __APP_PORT__ --path __HEALTH_PATH__; echo "exit $?"
```

### ✅ Verify

- [ ] `sudo nagios4 -v /etc/nagios4/nagios.cfg` reports `Total Errors: 0`.
- [ ] In the web UI, every host is **UP** and every service is **OK** (green).
- [ ] Stop the app on one server. The service goes **SOFT CRITICAL**, then **HARD CRITICAL** after `max_check_attempts`. Explain soft and hard states with this.
- [ ] Use the web UI to **schedule downtime**, **add a comment**, and **acknowledge** an alert (Practical 10).
- [ ] During `deploy.yml`, the host shows **in scheduled downtime**.

**If skipped:** remove the `nrpe` and `nagios` roles from `site.yml`, delete the downtime `pre_task` in `deploy.yml` (or skip it with `--skip-tags nagios`), and use `curl` in the deploy's post-task as the only health check.

---

## Stage 9: Secrets and security (always on)

**Purpose:** a pipeline that's safe to show publicly and that follows Unit IV's "best practices and security considerations".

### 🟡 OWNER CALL S9

- "Store AWS credentials with `aws configure` on your machine and in **GitHub Actions secrets** for any CI deploy?" *[yes]*
- "Encrypt Ansible secrets, such as the Nagios password, with **Ansible Vault**?" *[yes, if there are any secrets]*

### Checklist

- [ ] No secrets in git. `git log -p | grep -iE "aws_secret|BEGIN .*PRIVATE KEY|password="` returns nothing.
- [ ] AWS: use an IAM user or role with only EC2, VPC and S3 permissions, not root keys. Turn on MFA for the root account.
- [ ] `admin_cidr` is your IP `/32`. SSH and the ops UI are never open to `0.0.0.0/0`.
- [ ] App and NRPE ports are VPC-only. Only the load balancer is public.
- [ ] Ansible Vault for secrets: `ansible-vault create group_vars/vault.yml`, then run with `--ask-vault-pass`.
- [ ] Terraform state contains IPs and other details. Keep it out of git and in an encrypted S3 bucket (`encrypt = true`).
- [ ] Puppet certificate autosign is **off** outside the lab.
- [ ] `unattended-upgrades` is installed (the `common` role does this).
- [ ] The app runs as its own system user with systemd hardening (`ProtectSystem=strict`).

---

## Stage 10: Orchestration (Makefile)

**Purpose:** one memorable command for each step. This is great for the live demo.

### 🟡 OWNER CALL S10

- "Add a Makefile so the whole pipeline runs with commands like `make up` and `make down`?" *[yes]*

### Files

**`Makefile`**: remove the targets of skipped stages. Recipe lines must start with a **tab**, not spaces.

```makefile
TF      := infra/terraform
ANS     := infra/ansible
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)

.PHONY: help test build release infra inventory configure deploy puppet-run status up down

help:            ## list targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

test:            ## run app tests
	__TEST_CMD__

build:           ## build the artifact
	__BUILD_CMD__

release:         ## tag and push a release (make release VERSION=v1.2.0)
	git tag $(VERSION) && git push origin $(VERSION)

infra:           ## create servers with Terraform
	cd $(TF) && terraform init -input=false && terraform apply -auto-approve

configure:       ## install everything with Ansible
	cd $(ANS) && ansible-playbook site.yml -e app_version=$(VERSION)

deploy:          ## rolling deploy of VERSION
	cd $(ANS) && ansible-playbook deploy.yml -e app_version=$(VERSION)

puppet-run:      ## run Puppet on every agent now
	cd $(ANS) && ansible 'app:lb' -b -m ansible.builtin.command -a '/opt/puppetlabs/bin/puppet agent -t' || true

status:          ## show what is running
	cd $(TF) && terraform output
	cd $(ANS) && ansible app -b -m ansible.builtin.command -a 'systemctl is-active __APP_NAME__'

up: infra configure   ## everything from zero

down:            ## destroy all cloud resources
	cd $(TF) && terraform destroy
```

> `puppet agent -t` exits with code 2 when it changed something, so `|| true` stops Make treating that as a failure.
>
> When `deploy.yml` gets `app_version` from `-e`, Ansible skips the prompt.

### ✅ Verify

- [ ] `make help` lists every target.
- [ ] `make up` builds the full environment from nothing.

---

## Stage 11: Demo script and teardown

**Purpose:** a reliable live demo that shows every tool doing its job. Rehearse it twice.

### 🟡 OWNER CALL S11

- "Create everything live during the demo, or before it starts?" *[Before. `make up` can take 10–15 minutes.]*
- "Which failure to show: stop the app, or edit the nginx config?" *[both if there's time]*

### Demo runbook (about 10 minutes)

| # | Show | Command / action | What to say |
|---|---|---|---|
| 1 | The app working | Open `http://<lb ip>` and use the main feature | "This is the product." |
| 2 | CI | Push a small change, then open the Actions tab | "Every push is tested automatically." |
| 3 | Release | `make release VERSION=v1.1.0`, then show the Releases page | "A tag produces a versioned artifact." |
| 4 | Infrastructure as code | `terraform plan` reports *No changes* | "The servers are code. Running it again changes nothing: idempotency." |
| 5 | Rolling deploy | `make deploy VERSION=v1.1.0`, with a `curl` loop running beside it | "One server at a time, so no downtime. Nagios is put in downtime automatically." |
| 6 | Nagios dashboard | Everything green | "Health, disk, load and the real feature check." |
| 7 | **Break it** | `ssh app-1 sudo systemctl stop __APP_NAME__` | "Watch Nagios: soft critical, then hard critical. The load balancer still serves users from app-2." |
| 8 | **Self-heal** | `make puppet-run` | "Puppet pulls its desired state and starts the service again." |
| 9 | Back to green | Refresh Nagios | "Detected, healed and verified." |
| 10 | Drift on config | Edit the nginx file on lb, then `make puppet-run` | "Manual edits get reverted. The code is the source of truth." |
| 11 | Teardown | `make down` | "The whole environment is gone, and `make up` brings it back identically." |

### Teardown checklist

- [ ] `terraform destroy` finished: *Destroy complete!*
- [ ] AWS console → EC2: no running instances, no leftover volumes or Elastic IPs.
- [ ] `vagrant destroy -f` (if Vagrant was used).
- [ ] Source code and all infra code are still in git, so infrastructure can be destroyed without losing anything.

---

## Stage 12: Presentation

**Purpose:** present the **project first**, and the **pipeline second**, as the thing that makes it reliable.

### 🟡 OWNER CALL S12

- "How long is the presentation slot?" *[10–15 minutes]*
- "Live demo, recorded backup video, or both?" *[both. Record the runbook once as a fallback.]*

### Slide outline

1. **Title**: project name, your name, course (INT333)
2. **The problem**: who it's for and why it matters
3. **The product**: live demo of the main feature
4. **How it's built**: tech stack and a short architecture diagram
5. **The DevOps pipeline**: the stage map from the top of this file
6. **CI and releases**: Actions screenshot, Releases page
7. **Terraform (CO4)**: modules, workspaces, state, outputs → inventory
8. **Ansible (CO5, CO6)**: roles, handlers, vars, tags, prompt, rolling deploy
9. **Puppet (CO1, CO2)**: server/agent, certificates, module, template, facts, function, drift fixing
10. **Nagios (CO3)**: dashboard, custom plugin, soft/hard states, downtime
11. **Push vs pull**: why Ansible *and* Puppet (the table from S7)
12. **Security**: S9 checklist highlights
13. **Live demo**: the S11 runbook
14. **What I learned and what's next**

### Syllabus coverage map (put this on a slide)

| Course outcome | Where it appears in the project |
|---|---|
| **CO1** Basics of Puppet | Server/agent architecture, certificate authority, Facter, `puppet apply` in isolation |
| **CO2** Configuration management with Puppet | Module with classes, ERB template, custom facts, custom function, Hiera, drift fixing |
| **CO3** Nagios continuous monitoring | Package install, hosts/services/hostgroups, NRPE, custom plugin, web UI: downtime, comments, acknowledgements |
| **CO4** Terraform automation | AWS provisioning, modules, workspaces, remote state with locking, dependencies, outputs → Ansible |
| **CO5** Ansible fundamentals | Inventory, ad-hoc commands, Vagrant setup, architecture (agentless push) |
| **CO6** Ansible playbooks | Roles, handlers, variables, facts, prompts, tags, blocks, rolling deploy on AWS |

| Practical | Covered in stage |
|---|---|
| 1 Virtualization platform | S5B Vagrant + VirtualBox |
| 2–6 Puppet | S7 |
| 7–10 Nagios | S8 |
| 11–12 Ansible basics and playbooks | S6 |
| 13 Ansible with AWS | S6 on S5A |
| 14 Terraform | S5A |
| 15 Terraform + Ansible end-to-end | S5A `local_file` inventory → S6 |

---

## Appendix A: Owner-call quick sheet

Copy this into the project's issue tracker or notes, and tick the answers.

```text
S0  Target:        [ ] AWS   [ ] Vagrant   [ ] Existing servers
S0  Artifact:      [ ] binary   [ ] container   [ ] source
S3  CI:            [ ] include  [ ] skip      Infra lint: [ ] yes [ ] no
S4  Release:       [ ] include  [ ] skip      Platforms: ____________
S5  Remote state:  [ ] yes [ ] no             Workspaces: [ ] yes [ ] no
S6  Ansible:       [ ] include  [ ] skip      Rolling: [ ] yes [ ] no   Prompt: [ ] yes [ ] no
S6  Config owner:  [ ] puppet   [ ] ansible
S7  Puppet:        [ ] include  [ ] skip      Cert signing: [ ] manual [ ] autosign
S8  Nagios:        [ ] include  [ ] skip      Functional check: [ ] yes [ ] no
S8  UI access:     [ ] SSH tunnel   [ ] port 80 to my IP
S10 Makefile:      [ ] include  [ ] skip
S11 Live build:    [ ] before demo   [ ] during demo
S12 Slot length:   ____ min     Backup video: [ ] yes [ ] no
```

## Appendix B: Skip dependency matrix

| If you skip… | Then… |
|---|---|
| S3 CI | Run tests by hand before tagging |
| S4 Release | Ansible `app_install_method: copy` |
| S5 Infra | Write the inventory by hand (5B/5C) |
| S6 Ansible | Install OpenVox, Nagios and the app by hand, or have Puppet install the app |
| S7 Puppet | `config_owner: ansible`, `puppet_enabled: false`, and drop the `check_tcp 8140` service |
| S8 Nagios | `nagios_enabled: false`, and run deploys with `--skip-tags nagios` |
| ops node (S5) | Both S7 and S8 must be skipped, or run on an app server (not recommended) |
| Load balancer | Point Nagios's "Website" check at app-1, and rolling deploy will cause brief downtime |

## Appendix C: Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `terraform apply`: `UnauthorizedOperation` | IAM permissions missing | Add EC2 and VPC permissions to the IAM user |
| Ansible `UNREACHABLE` | Wrong key or IP, or SSH not allowed from your IP | Check `admin_cidr`, `ansible_ssh_private_key_file`; `ssh -v` |
| Ansible `get_url` 404 | Release tag or file name doesn't exist | Check the Releases page; `-e app_version=` must match the tag exactly |
| `puppet agent -t`: certificate errors | Not signed yet, or the agent was rebuilt | `puppetserver ca sign --all`. For a rebuilt agent: `puppetserver ca clean --certname <node>` on the server, delete `/etc/puppetlabs/puppet/ssl` on the agent, then run again |
| puppetserver won't start | Not enough RAM | Use `t3.small` or larger, and lower `JAVA_ARGS` (S7) |
| `node_role` fact is `unknown` | The `common` role didn't run | Run `site.yml` again, then check `cat /etc/node_role` |
| Nagios `Total Errors: 1+` | Typo in a `define` block | `sudo nagios4 -v /etc/nagios4/nagios.cfg` shows the line |
| NRPE: `CHECK_NRPE: Error - Could not connect` | `allowed_hosts` or security group | Make sure the ops private IP is in `allowed_hosts` and port 5666 is open inside the VPC |
| Nagios downtime task fails | External commands disabled | `check_external_commands=1` (S8 role) and restart nagios4 |
| Site shows 502 | App down on every backend | `systemctl status __APP_NAME__`; `journalctl -u __APP_NAME__ -n 50` |
| Unexpected AWS bill | Resources left running | `make down`; check EC2, EBS volumes and Elastic IPs in every region you used |

## Appendix D: What has been checked in this template

| Part | Check performed |
|---|---|
| All YAML blocks | Parsed without errors |
| Ansible `site.yml`, `deploy.yml` and roles | `ansible-playbook --syntax-check` and `ansible-lint` (production profile): 0 failures |
| Jinja templates (`lb.conf.j2`, `app.service.j2`, `project.cfg.j2`) | Rendered against a sample 4-host inventory |
| Nagios config generated by `project.cfg.j2` | `nagios4 -v` on Ubuntu: 0 warnings, 0 errors |
| `check_app.py` plugin | Tested OK, WARNING, CRITICAL (404 and refused) and UNKNOWN exit codes, plus a multipart upload |
| Ruby facts, Vagrantfile, ERB template | `ruby -c` syntax check; ERB rendered with sample data |
| Terraform, Puppet manifests | **Not executed.** Run `terraform validate` and `puppet parser validate` (or let CI's `infra` job do it) when you first fill the template in |

---

*End of template. Adapt freely, keep the owner calls, and keep the ✅ checks honest.*
