# fk DevOps pipeline

How fk is built, released, deployed, kept healthy and monitored. This pipeline was built from [`DEVOPS_PIPELINE_TEMPLATE.md`](DEVOPS_PIPELINE_TEMPLATE.md) using the template's defaults. The decisions are recorded in [`pipeline.config.yml`](../pipeline.config.yml).

```text
 git push ──► GitHub Actions CI ── gofmt, vet, tests, build + Terraform/Ansible/Puppet/Nagios checks
 git tag v* ─► GitHub Actions Release ── tests, builds fk-linux-amd64, publishes a GitHub Release
                     │
 make infra ─► Terraform (AWS ap-south-1) ── 4 EC2 servers + security groups ── writes Ansible inventory
                     │
 make configure ─► Ansible ── installs fk from the release, nginx, Nagios, NRPE, Puppet server/agents
                     │
 Puppet (every 30 min) ── keeps nginx config, fk service and cleanup cron in the declared state
                     │
 Nagios ── host up, SSH, disk, load, fk process, /healthz, and a real upload-and-compress check
```

## The servers

```text
                 Internet
                     │ :80
               ┌─────▼─────┐
               │   lb-1    │  nginx load balancer (config owned by Puppet)
               └─────┬─────┘
           ┌─────────┴─────────┐  :8000, VPC only
      ┌────▼────┐         ┌────▼────┐
      │  app-1  │         │  app-2  │  fk api (systemd) + NRPE + Puppet agent
      └────┬────┘         └────┬────┘
           └─────────┬─────────┘  :5666 NRPE, :8140 Puppet, VPC only
               ┌─────▼─────┐
               │   ops-1   │  Puppet server (OpenVox 8) + Nagios 4
               └───────────┘  web UI only through an SSH tunnel
```

## What each tool does here

| Tool | Files | Job in this project | Course outcome |
|---|---|---|---|
| GitHub Actions | `.github/workflows/` | Tests every push, publishes a release for every `v*` tag | — |
| Terraform | `infra/terraform/` | Creates the network rules and 4 servers. Uses modules and workspaces, supports S3 remote state, and writes `inventory/hosts.ini` | CO4 |
| Ansible | `infra/ansible/` | **Push**: installs everything, and runs rolling deploys one server at a time with Nagios downtime | CO5, CO6 |
| Puppet | `infra/puppet/` | **Pull**: every 30 minutes it puts back the nginx config, restarts fk if it's stopped, and runs the cleanup cron | CO1, CO2 |
| Nagios | `infra/nagios/` + `roles/nagios` | Monitors all 4 hosts; the custom plugin uploads a real JPEG to fk and checks a JPEG comes back | CO3 |
| Vagrant | `Vagrantfile` | The same 4 servers as local VirtualBox VMs | Practical 1 |

## Before you start (once)

On your Ubuntu/WSL machine:

```bash
# Tools
sudo apt install -y make python3-pip unzip
pip install --user ansible            # Ansible
# Terraform: you already installed it for the INT333 practical (terraform --version)
# AWS CLI v2: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html

# An SSH key for the servers (press Enter for every question)
ssh-keygen -t ed25519

# AWS access: an IAM user's access key, never the root account
aws configure                          # region: ap-south-1

# Your IP, so only you can SSH in
cd infra/terraform
cp example.tfvars terraform.tfvars
echo "admin_cidr = \"$(curl -s https://checkip.amazonaws.com)/32\"" > terraform.tfvars
```

## Run it

| Step | Command | What happens |
|---|---|---|
| 1 | `make release VERSION=v1.1.0` | Tags the code. GitHub Actions tests it and publishes `fk-linux-amd64` |
| 2 | `cd infra/terraform && terraform workspace new dev && cd -` | Creates the `dev` workspace (first time only) |
| 3 | `make up VERSION=v1.1.0` | Terraform creates the servers, Ansible configures them, Puppet certificates are signed, and Puppet runs. Takes 10–15 minutes |
| 4 | `cd infra/terraform && terraform output` | Prints `site_url` (open it) and the `nagios_tunnel` command |
| 5 | Run the `nagios_tunnel` command, then open http://localhost:8090/nagios4 | Nagios dashboard |
| 6 | `make deploy VERSION=v1.2.0` | Rolling upgrade of a newer release, one app server at a time |
| 7 | `make down` | Deletes every AWS resource. **Always do this when you're finished** |

### Local version (no AWS)

Needs VirtualBox and Vagrant on a machine with about 6 GB of free RAM.

```bash
make vagrant-up
make vagrant-configure VERSION=v1.1.0
# site:   http://192.168.56.10
# Nagios: ssh -L 8090:localhost:80 -i .vagrant/machines/ops-1/virtualbox/private_key vagrant@192.168.56.20
make vagrant-down
```

## Demo runbook (about 10 minutes)

| # | Show | Do | Say |
|---|---|---|---|
| 1 | The product | Open `site_url`, compress a PDF and convert a photo | "fk compresses and converts files. This is the hosted version." |
| 2 | The CLI | `fk` in a folder, Space to select, Enter | "The same engine as a terminal tool." |
| 3 | CI | GitHub → Actions | "Every push runs format checks, vet, tests and infrastructure lint." |
| 4 | Release | GitHub → Releases | "A tag builds and publishes a versioned binary." |
| 5 | Terraform | `terraform plan` → *No changes* | "Servers are code, and re-running changes nothing." |
| 6 | Rolling deploy | `make deploy VERSION=v1.2.0` with `while true; do curl -s -o /dev/null -w "%{http_code} " http://<lb>/healthz; sleep 1; done` running | "One server at a time: the site never goes down, and Nagios is muted for each server while it updates." |
| 7 | Nagios | All green, including **fk real function** | "This check uploads a real image and expects a compressed one back." |
| 8 | Break it | `ssh ubuntu@<app-1> sudo systemctl stop fk` | "Nagios goes SOFT, then HARD CRITICAL. The load balancer keeps users on app-2." |
| 9 | Self-heal | `make puppet-run` | "Puppet pulls the desired state and starts fk again." |
| 10 | Drift | `ssh ubuntu@<lb-1>`, edit `/etc/nginx/sites-available/default`, then `make puppet-run` | "Manual edits are reverted. The code is the source of truth." |
| 11 | Teardown | `make down` | "Everything is gone, and `make up` rebuilds it identically." |

## How the course syllabus is covered

| Outcome | Where |
|---|---|
| CO1 Puppet basics | Server/agent, certificate signing (`make puppet-sign`), Facter, `puppet apply` |
| CO2 Puppet configuration management | Module `fk` with classes `base`, `app`, `lb`; ERB template; custom facts `node_role` and `app_version`; custom function `fk::backend_list`; Hiera data |
| CO3 Nagios | `nagios4` package, hosts/hostgroups/services, NRPE, custom plugin `check_app.py`, soft/hard states, downtime from Ansible |
| CO4 Terraform | AWS provider, `network` and `compute` modules, workspaces, S3 remote state with locking, outputs → inventory |
| CO5 Ansible basics | Inventory, ad-hoc commands (`make status`), Vagrant |
| CO6 Ansible playbooks | Roles, handlers, variables, facts, `vars_prompt`, tags, `block`/`rescue`, `serial: 1` rolling deploy |

| Practical | Where |
|---|---|
| 1 Virtualization | `Vagrantfile` |
| 2–6 Puppet | `infra/puppet/`, roles `puppet_server` and `puppet_agent` |
| 7–10 Nagios | role `nagios`, role `nrpe`, `infra/nagios/plugins/check_app.py` |
| 11–13 Ansible | `infra/ansible/` on AWS |
| 14 Terraform | `infra/terraform/` |
| 15 Terraform + Ansible | `local_file.ansible_inventory` → `site.yml` |

## Cost and safety

- Instance sizes: `t3.micro` for lb-1, app-1 and app-2; `t3.small` for ops-1 (Puppet server needs the memory). Run `make down` after every session.
- SSH is open only to `admin_cidr`. App, NRPE and Puppet ports are reachable only inside the VPC. Only lb-1 port 80 is public.
- fk api keeps nothing: each upload is processed in its own temporary folder, which is deleted right after the response. Puppet's cron removes anything older than 60 minutes as a backstop.
- Secrets (AWS keys, SSH private keys, `terraform.tfvars`, state) are git-ignored.

Troubleshooting: see Appendix C of the [template](DEVOPS_PIPELINE_TEMPLATE.md#appendix-c-troubleshooting).
