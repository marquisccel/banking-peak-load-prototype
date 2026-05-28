#!/usr/bin/env python3
"""
Dynamic inventory for Ansible. Reads Terraform outputs from:
deployments/terraform/cloud-demo

Expected Terraform outputs:
- app_public_ip
- k6_public_ip
- api_url
"""

import json
import os
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
TERRAFORM_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, "../../terraform/cloud-demo"))
SSH_KEY = os.path.expanduser(os.environ.get("ANSIBLE_SSH_KEY", "~/.ssh/id_rsa"))


def fail(message: str) -> None:
    print(message, file=sys.stderr)
    sys.exit(1)


def get_terraform_outputs() -> dict:
    if not os.path.isdir(TERRAFORM_DIR):
        fail(f"Terraform directory not found: {TERRAFORM_DIR}")

    result = subprocess.run(
        ["terraform", "output", "-json"],
        cwd=TERRAFORM_DIR,
        capture_output=True,
        text=True,
        check=False,
    )

    if result.returncode != 0:
        fail(
            "Failed to read Terraform outputs. Run `terraform apply` first in "
            f"{TERRAFORM_DIR}.\n{result.stderr.strip()}"
        )

    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        fail(f"Invalid Terraform JSON output: {exc}")


def output_value(outputs: dict, name: str):
    if name not in outputs or "value" not in outputs[name]:
        fail(f"Missing Terraform output: {name}")
    value = outputs[name]["value"]
    if value in (None, ""):
        fail(f"Terraform output is empty: {name}")
    return value


def build_inventory(outputs: dict) -> dict:
    app_ip = output_value(outputs, "app_public_ip")
    k6_ip = output_value(outputs, "k6_public_ip")
    api_url = output_value(outputs, "api_url")

    hostvars = {
        "app_server": {
            "ansible_host": app_ip,
            "api_url": api_url,
            "grafana_url": f"http://{app_ip}:3000",
            "prometheus_url": f"http://{app_ip}:9090",
        },
        "k6_runner": {
            "ansible_host": k6_ip,
            "app_base_url": api_url.rstrip("/"),
        },
    }

    return {
        "all": {
            "vars": {
                "ansible_user": "ubuntu",
                "ansible_ssh_private_key_file": SSH_KEY,
                "ansible_ssh_common_args": "-o StrictHostKeyChecking=no",
            },
            "children": {
                "app_servers": {"hosts": {"app_server": {}}},
                "k6_runners": {"hosts": {"k6_runner": {}}},
            },
        },
        "_meta": {"hostvars": hostvars},
    }


if __name__ == "__main__":
    print(json.dumps(build_inventory(get_terraform_outputs()), indent=2))
