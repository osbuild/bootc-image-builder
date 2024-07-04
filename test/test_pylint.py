import pathlib
import subprocess


def test_pylint():
    p = pathlib.Path(__file__).parent
    subprocess.check_call(
        ["pylint",
         "--disable=fixme",
         "--disable=missing-class-docstring",
         "--disable=missing-module-docstring",
         "--disable=missing-function-docstring",
         "--disable=too-many-instance-attributes",
         # false positive because of "if yield else yield" in
         # the "build_container" fixture, see
         # https://pylint.readthedocs.io/en/latest/user_guide/messages/warning/contextmanager-generator-missing-cleanup.html
         "--disable=contextmanager-generator-missing-cleanup",
         "--max-line-length=120"] + list(p.glob("*.py")))
