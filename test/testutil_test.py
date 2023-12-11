import contextlib
import subprocess
from unittest.mock import call, patch

import pytest

from testutil import has_executable, get_free_port, wait_ssh_ready


def test_get_free_port():
    port_nr = get_free_port()
    assert port_nr > 1024 and port_nr < 65535


@pytest.fixture(name="free_port")
def free_port_fixture():
    return get_free_port()


@patch("time.sleep")
def test_wait_ssh_ready_sleeps_no_connection(mocked_sleep, free_port):
    with pytest.raises(ConnectionRefusedError):
        wait_ssh_ready(free_port, sleep=0.1, max_wait_sec=0.35)
    assert mocked_sleep.call_args_list == [call(0.1), call(0.1), call(0.1)]


def test_wait_ssh_ready_sleeps_wrong_reply(free_port, tmp_path):
    with contextlib.ExitStack() as cm:
        p = subprocess.Popen(
            f"echo not-ssh | nc -v -l {free_port}",
            shell=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            encoding="utf-8",
        )
        cm.callback(p.kill)
        # wait for nc to be ready
        while True:
            if "Listening " in p.stdout.readline():
                break
        # now connect
        with patch("time.sleep") as mocked_sleep:
            with pytest.raises(ConnectionRefusedError):
                wait_ssh_ready(free_port, sleep=0.1, max_wait_sec=0.55)
            assert mocked_sleep.call_args_list == [
                call(0.1), call(0.1), call(0.1), call(0.1), call(0.1)]


@pytest.mark.skipif(not has_executable("nc"), reason="needs nc")
def test_wait_ssh_ready_integration(free_port, tmp_path):
    with contextlib.ExitStack() as cm:
        p = subprocess.Popen(f"echo OpenSSH | nc -l {free_port}", shell=True)
        cm.callback(p.kill)
        wait_ssh_ready(free_port, sleep=0.1, max_wait_sec=10)
