import socket
import shutil
import subprocess
import time


def journal_cursor():
    output = subprocess.check_output(["journalctl", "-n0", "--show-cursor"], encoding="utf-8").strip()
    cursor = output.split("\n")[-1]
    return cursor.split("cursor: ")[-1]


def journal_after_cursor(cursor):
    output = subprocess.check_output(["journalctl", f"--after-cursor={cursor}"], encoding="utf8")
    return output


def has_executable(name):
    return shutil.which(name) is not None


def get_free_port() -> int:
    # this is racy but there is no race-free way to do better with the qemu CLI
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("localhost", 0))
        return s.getsockname()[1]


def wait_ssh_ready(port, sleep, max_wait_sec):
    for i in range(int(max_wait_sec / sleep)):
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(sleep)
            try:
                s.connect(("localhost", port))
                data = s.recv(256)
                if b"OpenSSH" in data:
                    return
            except (ConnectionRefusedError, TimeoutError):
                time.sleep(sleep)
    raise ConnectionRefusedError(f"cannot connect to port {port} after {max_wait_sec}s")
