#!/usr/bin/env python3
import json
import os
import subprocess
import sys

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
TERRAFORM_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, "../../terraform/cloud-demo"))
SSH_KEY = os.path.expanduser(os.environ.get("ANSIBLE_SSH_KEY", "~/.ssh/id_rsa"))

def get_terraform_outputs():
    result = subprocess.run(
        ["terraform", "output", "-json"],
        cwd=TERRAFORM_DIR,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        print(f"Error: {result.stderr}", file=sys.stderr)
        sys.exit(1)
    return json.loads(result.stdout)

def build_inventory(outputs):
    app_ip  = outputs["app_public_ip"]["value"]
    k6_ip   = outputs["k6_public_ip"]["value"]
    api_url = outputs["api_url"]["value"]

    return {
        "app_servers": {
            "hosts": ["app_server"]
        },
        "k6_runners": {
            "hosts": ["k6_runner"]
        },
        "_meta": {
            "hostvars": {
                "app_server": {
                    "ansible_host": app_ip,
                    "ansible_user": "ubuntu",
                    "ansible_ssh_private_key_file": SSH_KEY,
                    "api_url": api_url,
                    "grafana_url": f"http://{app_ip}:3000",
                    "prometheus_url": f"http://{app_ip}:9090"
                },
                "k6_runner": {
                    "ansible_host": k6_ip,
                    "ansible_user": "ubuntu",
                    "ansible_ssh_private_key_file": SSH_KEY,
                    "app_base_url": api_url
                }
            }
        }
    }

if __name__ == "__main__":
    outputs = get_terraform_outputs()
    inventory = build_inventory(outputs)
    print(json.dumps(inventory, indent=2))
