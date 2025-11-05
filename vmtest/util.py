import socket
import time


def get_free_port() -> int:
    # this is racy but there is no race-free way to do better with the qemu CLI
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("localhost", 0))
        return s.getsockname()[1]


def wait_ssh_ready(address, port, sleep, max_wait_sec):
    for _ in range(int(max_wait_sec / sleep)):
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
