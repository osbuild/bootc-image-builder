import os
import pathlib
import platform
import shutil
import socket
import subprocess
import time

import boto3
from botocore.exceptions import ClientError

AWS_REGION = "us-east-1"


def run_journalctl(*args):
    pre = []
    if platform.system() == "Darwin":
        pre = ["podman", "machine", "ssh"]
    cmd = pre + ["journalctl"] + list(args)
    return subprocess.check_output(cmd, encoding="utf-8").strip()


def journal_cursor():
    output = run_journalctl("-n0", "--show-cursor")
    cursor = output.split("\n")[-1]
    return cursor.split("cursor: ")[-1]


def journal_after_cursor(cursor):
    output = run_journalctl(f"--after-cursor={cursor}")
    return output


def has_executable(name):
    return shutil.which(name) is not None


def get_free_port() -> int:
    # this is racy but there is no race-free way to do better with the qemu CLI
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("localhost", 0))
        return s.getsockname()[1]


def wait_ssh_ready(address, port, sleep, max_wait_sec):
    for i in range(int(max_wait_sec / sleep)):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(sleep)
            try:
                s.connect((address, port))
                data = s.recv(256)
                if b"OpenSSH" in data:
                    return
            except (ConnectionRefusedError, ConnectionResetError, TimeoutError):
                pass
            time.sleep(sleep)
    raise ConnectionRefusedError(f"cannot connect to port {port} after {max_wait_sec}s")


def has_x86_64_v3_cpu():
    # x86_64-v3 has multiple features, see
    # https://en.wikipedia.org/wiki/X86-64#Microarchitecture_levels
    # but "avx2" is probably a good enough proxy
    return " avx2 " in pathlib.Path("/proc/cpuinfo").read_text()


def can_start_rootful_containers():
    system = platform.system()
    if system == "Linux":
        # on linux we need to run "podman" with sudo to get full
        # root containers
        return os.getuid() == 0
    elif system == "Darwin":
        # on darwin a container is root if the podman machine runs
        # in "rootful" mode, i.e. no need to run "podman" as root
        # as it's just proxying to the VM
        res = subprocess.run([
            "podman", "machine", "inspect", "--format={{.Rootful}}",
        ], capture_output=True, encoding="utf8", check=True)
        return res.stdout.strip() == "true"
    else:
        raise ValueError(f"unknown platform {system}")


def write_aws_creds(path):
    key_id = os.environ.get("AWS_ACCESS_KEY_ID")
    secret_key = os.environ.get("AWS_SECRET_ACCESS_KEY")
    if not key_id or not secret_key:
        return False

    with open(path, mode="w", encoding="utf-8") as creds_file:
        creds_file.write("[default]\n")
        creds_file.write(f"aws_access_key_id = {key_id}\n")
        creds_file.write(f"aws_secret_access_key = {secret_key}\n")

    return True


def deregister_ami(ami_id):
    ec2 = boto3.resource("ec2", region_name=AWS_REGION)
    try:
        print(f"Deregistering image {ami_id}")
        ami = ec2.Image(ami_id)
        ami.deregister()
        print("Image deregistered")
    except ClientError as err:
        err_code = err.response["Error"]["Code"]
        err_msg = err.response["Error"]["Message"]
        print(f"Couldn't deregister image {ami_id}.")
        print(f"Error {err_code}: {err_msg}")
