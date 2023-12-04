import shutil
import subprocess


def journal_cursor():
    output = subprocess.check_output(["journalctl", "-n0", "--show-cursor"], encoding="utf-8").strip()
    cursor = output.split("\n")[-1]
    return cursor.split("cursor: ")[-1]


def journal_after_cursor(cursor):
    output = subprocess.check_output(["journalctl", f"--after-cursor={cursor}"])
    return output


def has_executable(name):
    return shutil.which(name) is not None
