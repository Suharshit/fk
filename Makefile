# fk — build and DevOps pipeline commands.  Run `make help` for the list.
# Recipe lines must start with a TAB.

TF      := infra/terraform
ANS     := infra/ansible
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0)
LDFLAGS := -s -w -X main.version=$(VERSION)
# Ansible inventory: Terraform writes hosts.ini; use INV=inventory/vagrant.ini for local VMs
INV     ?= inventory/hosts.ini

.PHONY: help test lint build run-api release infra configure deploy puppet-sign puppet-run \
        status up down vagrant-up vagrant-configure vagrant-down

help:              ## list targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

# ---------- app ----------

test:              ## run the Go tests
	go test ./...

lint:              ## gofmt + go vet
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)
	go vet ./...

build:             ## build dist/fk-linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/fk-linux-amd64 .

run-api: build     ## run the web app locally on :8000
	./dist/fk-linux-amd64 api --port 8000

release:           ## tag and push a release: make release VERSION=v1.2.0
	git tag $(VERSION) && git push origin $(VERSION)

# ---------- infrastructure (AWS) ----------

infra:             ## create the servers with Terraform (writes the Ansible inventory)
	cd $(TF) && terraform init -input=false && terraform apply -auto-approve

configure:         ## install everything with Ansible
	cd $(ANS) && ansible-galaxy collection install -r requirements.yml >/dev/null && \
	  ansible-playbook -i $(INV) site.yml -e app_version=$(VERSION)

puppet-sign:       ## sign the Puppet agents' certificates on the ops server
	sleep 15
	cd $(ANS) && ansible -i $(INV) ops -b -m ansible.builtin.command \
	  -a '/opt/puppetlabs/bin/puppetserver ca sign --all' || true

puppet-run:        ## run Puppet on every agent now (exit code 2 = changes made)
	cd $(ANS) && ansible -i $(INV) 'app:lb' -b -m ansible.builtin.command \
	  -a '/opt/puppetlabs/bin/puppet agent -t' || true

deploy:            ## rolling deploy of VERSION: make deploy VERSION=v1.2.0
	cd $(ANS) && ansible-playbook -i $(INV) deploy.yml -e app_version=$(VERSION)

status:            ## show addresses and whether fk is running
	-cd $(TF) && terraform output
	cd $(ANS) && ansible -i $(INV) app -b -m ansible.builtin.command -a 'systemctl is-active fk'

up: infra configure puppet-sign puppet-run   ## everything from zero on AWS

down:              ## destroy all AWS resources
	cd $(TF) && terraform destroy

# ---------- local VMs (Vagrant + VirtualBox) ----------

vagrant-up:        ## start the 4 local VMs
	vagrant up

vagrant-configure: ## configure the local VMs with Ansible
	$(MAKE) configure puppet-sign puppet-run INV=inventory/vagrant.ini

vagrant-down:      ## delete the local VMs
	vagrant destroy -f
