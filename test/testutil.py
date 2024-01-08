import os
import pathlib
import platform
import shutil
import socket
import subprocess
import time

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
    match platform.system():
        case "Linux":
            # on linux we need to run "podman" with sudo to get full
            # root containers
            return os.getuid() == 0
        case "Darwin":
            # on darwin a container is root if the podman machine runs
            # in "rootful" mode, i.e. no need to run "podman" as root
            # as it's just proxying to the VM
            res = subprocess.run([
                "podman", "machine", "inspect", "--format={{.Rootful}}",
            ], capture_output=True, encoding="utf8", check=True)
            return res.stdout.strip() == "true"
        case unknown:
            raise ValueError(f"unknown platform {unknown}")
