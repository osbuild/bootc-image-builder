import abc
import pathlib
import subprocess
import sys
from io import StringIO

from paramiko.client import AutoAddPolicy, SSHClient

from testutil import get_free_port, wait_ssh_ready


class VM(abc.ABC):

    def __init__(self):
        self._ssh_port = None
        self._address = None

    def __del__(self):
        self.force_stop()

    @abc.abstractmethod
    def start(self):
        """
        Start the VM. This method will be called automatically if it is not called explicitly before calling run().
        """

    def _log(self, msg):
        # XXX: use a proper logger
        sys.stdout.write(msg.rstrip("\n") + "\n")

    def wait_ssh_ready(self):
        wait_ssh_ready(self._address, self._ssh_port, sleep=1, max_wait_sec=600)

    @abc.abstractmethod
    def force_stop(self):
        """
        Stop the VM and clean up any resources that were created when setting up and starting the machine.
        """

    @abc.abstractmethod
    def run(self, cmd, user, password):
        """
        Run a command on the VM via SSH using the provided credentials.
        """

    def __enter__(self):
        self.start()
        return self

    def __exit__(self, type, value, tb):
        self.force_stop()


class QEMU(VM):

    MEM = "2000"
    # TODO: support qemu-system-aarch64 too :)
    QEMU = "qemu-system-x86_64"

    def __init__(self, img, snapshot=True):
        super().__init__()
        self._img = pathlib.Path(img)
        self._qemu_p = None
        self._snapshot = snapshot

    def start(self):
        if self._qemu_p is not None:
            return
        log_path = self._img.with_suffix(".serial-log")
        self._ssh_port = get_free_port()
        self._address = "localhost"
        qemu_cmdline = [
            self.QEMU, "-enable-kvm",
            "-m", self.MEM,
            # get "illegal instruction" inside the VM otherwise
            "-cpu", "host",
            "-nographic",
            "-serial", "stdio",
            "-monitor", "none",
            "-netdev", f"user,id=net.0,hostfwd=tcp::{self._ssh_port}-:22",
            "-device", "rtl8139,netdev=net.0",
        ]
        if self._snapshot:
            qemu_cmdline.append("-snapshot")
        qemu_cmdline.append(self._img)
        self._log(f"vm starting, log available at {log_path}")

        # XXX: use systemd-run to ensure cleanup?
        self._qemu_p = subprocess.Popen(
            qemu_cmdline, stdout=sys.stdout, stderr=sys.stderr)
        # XXX: also check that qemu is working and did not crash
        self.wait_ssh_ready()
        self._log(f"vm ready at port {self._ssh_port}")

    def force_stop(self):
        if self._qemu_p:
            self._qemu_p.kill()
            self._qemu_p = None
            self._address = None
            self._ssh_port = None

    def run(self, cmd, user, password):
        if not self._qemu_p:
            self.start()
        client = SSHClient()
        client.set_missing_host_key_policy(AutoAddPolicy)
        client.connect(
            self._address, self._ssh_port, user, password,
            allow_agent=False, look_for_keys=False)
        chan = client.get_transport().open_session()
        chan.get_pty()
        chan.exec_command(cmd)
        stdout_f = chan.makefile()
        output = StringIO()
        while True:
            out = stdout_f.readline()
            if not out:
                break
            self._log(out)
            output.write(out)
        exit_status = stdout_f.channel.recv_exit_status()
        return exit_status, output.getvalue()
