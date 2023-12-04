import os
import pathlib
import subprocess


def test_flake8():
    p = pathlib.Path(__file__).parent
    # TODO: use all static checks from osbuild instead
    subprocess.check_call(
        ["flake8", "--ignore=E402", "--max-line-length=120",
         os.fspath(p)])
