# Cloud Demo Runbook

This is the canonical runbook for the AWS cloud load test demo. Follow it from top to bottom from a WSL/Linux shell.

## What This Creates

- App server: Banking API, PostgreSQL, Redis, RabbitMQ, PgBouncer, Prometheus, and Grafana.
- k6 runner: remote load generator that runs the k6 scripts through Ansible.

## 0. Start From A Clean Local Repo

Run the demo from the Linux home directory, not from `/mnt/...`.

```bash
cd ~

# Clone if this is a fresh WSL/Linux machine.
git clone https://github.com/egayurcel990/banking-peak-load-prototype.git

# Alternative: if the repo is still on the Windows drive and does not exist in ~/ yet.
# cp -r /mnt/d/path/to/banking-peak-load-prototype ~/banking-peak-load-prototype

cd ~/banking-peak-load-prototype
git pull origin main
```

If `git pull` refuses because `deployments/ansible/inventories/terraform_inventory.py` has local changes, keep the local change safe first:

```bash
git stash push -m "local terraform inventory change" deployments/ansible/inventories/terraform_inventory.py
git pull origin main
```

## 1. Prerequisites

```bash
aws sts get-caller-identity
terraform version
ansible --version
```

If the SSH key does not exist:

```bash
ssh-keygen -t rsa -b 4096 -f ~/.ssh/id_rsa -N ""
```

If Ansible is missing:

```bash
pip install ansible
```

## 2. Provision EC2 With Terraform

Always run Terraform from `deployments/terraform/cloud-demo`.

```bash
cd ~/banking-peak-load-prototype/deployments/terraform/cloud-demo
cp -n terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

```hcl
aws_region        = "us-east-1"
repo_url          = "https://github.com/egayurcel990/banking-peak-load-prototype.git"
public_key_path   = "~/.ssh/id_rsa.pub"
ssh_cidr          = "<your-public-ip>/32"
public_access_cidr = "0.0.0.0/0"
```

Get your public IP:

```bash
curl -4 ifconfig.me
```

Apply:

```bash
terraform init
terraform apply
terraform output
```

If SSH later becomes unreachable, your public IP probably changed. Re-apply the SSH CIDR from this same Terraform directory:

```bash
terraform apply -var="ssh_cidr=$(curl -4 -s ifconfig.me)/32"
```

Do not run `terraform apply` from `deployments/ansible`; Terraform will fail with `No configuration files`.

## 3. Verify Ansible Inventory And SSH

Move from the Terraform directory to the Ansible directory:

```bash
cd ../../ansible
chmod +x inventories/terraform_inventory.py
ansible all -i inventories/terraform_inventory.py -m ping
```

Expected result:

```text
app_server | SUCCESS
k6_runner  | SUCCESS
```

If Ansible says `Permission denied` for `terraform_inventory.py`, run:

```bash
chmod +x inventories/terraform_inventory.py
```

If Ansible says `UNREACHABLE` on port 22, update `ssh_cidr`:

```bash
cd ../terraform/cloud-demo
terraform apply -var="ssh_cidr=$(curl -4 -s ifconfig.me)/32"
cd ../../ansible
ansible all -i inventories/terraform_inventory.py -m ping
```

## 4. Configure Servers With Ansible

For a fresh EC2 demo or after `terraform destroy`:

```bash
ansible-playbook -i inventories/terraform_inventory.py site.yml -e seed=true
```

For a repeat demo on the same running EC2 instances where data already exists:

```bash
ansible-playbook -i inventories/terraform_inventory.py site.yml
```

Use `seed=true` only when the database is new, empty, or you intentionally want to reset to 100K accounts and 1M transactions.

Expected recap:

```text
unreachable=0
failed=0
```

## 5. Verify URLs And Dashboards

From `deployments/ansible`:

```bash
terraform -chdir=../terraform/cloud-demo output -raw api_url
terraform -chdir=../terraform/cloud-demo output -raw grafana_url
terraform -chdir=../terraform/cloud-demo output -raw prometheus_url
```

Open Grafana and log in:

```text
admin / admin
```

## 6. Run The Cloud Load Test

Check status first:

```bash
ansible-playbook -i inventories/terraform_inventory.py loadtest.yml -e loadtest_script=run-status.sh
```

Run the default mixed load test:

```bash
ansible-playbook -i inventories/terraform_inventory.py loadtest.yml
```

Default mixed load test settings:

```text
300 iterations/s
10 minutes
70% reads / 30% writes
```

The playbook stops any previous `k6 run` process before starting a new test, so two 300 iter/s tests do not overlap into ~600 req/s.

Expected k6 output:

```text
mixed_peak_load: 300.00 iterations/s for 10m0s
iterations: ... 299.xxx/s
http_reqs: ... 299.xxx/s
```

Optional variants:

```bash
ansible-playbook -i inventories/terraform_inventory.py loadtest.yml -e loadtest_script=run-spike.sh
ansible-playbook -i inventories/terraform_inventory.py loadtest.yml -e loadtest_script=run-optimized.sh
```

## 7. Update Existing EC2 Instances After A Git Pull

If you pulled new Ansible or k6 template changes after the instances were already configured:

```bash
cd ~/banking-peak-load-prototype
git pull origin main
cd deployments/ansible
chmod +x inventories/terraform_inventory.py
ansible-playbook -i inventories/terraform_inventory.py deploy.yml
```

This refreshes the app repo and k6 runner scripts without recreating EC2.

## 8. Stop Any Running k6 Test Manually

Use this only if a demo is currently running and you need to stop it:

```bash
ansible k6_runners -i inventories/terraform_inventory.py -m shell -a "pkill -f '[k]6 run' || true"
```

Check that no k6 process is still running:

```bash
ansible k6_runners -i inventories/terraform_inventory.py -m shell -a "pgrep -af 'k6 run' || true"
```

## 9. Destroy

Destroy from the Terraform directory:

```bash
cd ~/banking-peak-load-prototype/deployments/terraform/cloud-demo
terraform destroy
```

Confirm with:

```text
yes
```

After destroy, the API, Grafana, Prometheus, SSH, and k6 runner URLs will stop working.

## Common Errors

| Error | Cause | Fix |
|---|---|---|
| `terraform apply` says `No configuration files` | You are in `deployments/ansible` | `cd ~/banking-peak-load-prototype/deployments/terraform/cloud-demo` |
| `cd ansible: No such file or directory` | You are in `deployments/terraform`; Ansible is sibling of Terraform | `cd ../ansible` from `deployments`, or `cd ../../ansible` from `deployments/terraform/cloud-demo` |
| Ansible inventory `Permission denied` | `terraform_inventory.py` is not executable after pull/copy | `chmod +x deployments/ansible/inventories/terraform_inventory.py` |
| `UNREACHABLE` on port 22 | AWS Security Group SSH CIDR does not include current public IP | `terraform apply -var="ssh_cidr=$(curl -4 -s ifconfig.me)/32"` |
| `loadtest.yml could not be found` | Local WSL repo has not pulled latest main | `cd ~/banking-peak-load-prototype && git pull origin main` |
| `git pull` blocked by local inventory changes | Local file differs from GitHub | `git stash push -m "local terraform inventory change" deployments/ansible/inventories/terraform_inventory.py` then `git pull origin main` |
| Grafana shows ~600 req/s | Two k6 runs overlapped | Re-run `loadtest.yml`; it now stops previous `k6 run` first |
| Grafana shows no data | Prometheus has not scraped or no load has run yet | Run status/load test and wait 2-3 minutes |
