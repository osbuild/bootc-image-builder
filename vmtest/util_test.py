import contextlib
import shutil
import subprocess
from unittest.mock import call, patch

import pytest

from vmtest.util import get_free_port, wait_ssh_ready


def test_get_free_port():
    port_nr = get_free_port()
    assert 1024 < port_nr < 65535


@patch("time.sleep")
def test_wait_ssh_ready_sleeps_no_connection(mocked_sleep):
    free_port = get_free_port()
    with pytest.raises(ConnectionRefusedError):
        wait_ssh_ready("localhost", free_port, sleep=0.1, max_wait_sec=0.35)
    assert mocked_sleep.call_args_list == [call(0.1), call(0.1), call(0.1)]


@pytest.mark.skipif(not shutil.which("nc"), reason="needs nc")
def test_wait_ssh_ready_sleeps_wrong_reply():
    free_port = get_free_port()
    with contextlib.ExitStack() as cm:
        with subprocess.Popen(
            f"echo not-ssh | nc -vv -l -p {free_port}",
            shell=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            encoding="utf-8",
        ) as p:
            cm.callback(p.kill)
            # wait for nc to be ready
            while True:
                # netcat tranditional uses "listening", others "Listening"
                # so just omit the first char
                if "istening " in p.stdout.readline():
                    break
            # now connect
            with patch("time.sleep") as mocked_sleep:
                with pytest.raises(ConnectionRefusedError):
                    wait_ssh_ready("localhost", free_port, sleep=0.1, max_wait_sec=0.55)
                assert mocked_sleep.call_args_list == [
                    call(0.1), call(0.1), call(0.1), call(0.1), call(0.1)]
