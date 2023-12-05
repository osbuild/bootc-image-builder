import contextlib
import subprocess
import time
from unittest.mock import call, patch

import pytest

from testutil import has_executable, get_free_port, wait_ssh_ready


def test_get_free_port():
    port_nr = get_free_port()
    assert port_nr > 1024 and port_nr < 65535


@pytest.mark.skipif(not has_executable("nc"), reason="needs nc")
@patch("time.sleep", wraps=time.sleep)
def test_wait_ssh_ready(mocked_sleep):
    port = get_free_port()
    with pytest.raises(ConnectionRefusedError):
        wait_ssh_ready(port, sleep=0.1, max_wait_sec=0.35)
    assert mocked_sleep.call_args_list == [call(0.1), call(0.1), call(0.1)]
    # now make port ready
    with contextlib.ExitStack() as cm:
        p = subprocess.Popen(f"echo OpenSSH | nc -l {port}", shell=True)
        cm.callback(p.kill)
        wait_ssh_ready(port, sleep=0.1, max_wait_sec=10)
