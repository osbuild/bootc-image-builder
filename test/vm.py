import abc
import os
import pathlib
import subprocess
import sys
import time
import uuid
from io import StringIO

import boto3
from botocore.exceptions import ClientError
from paramiko.client import AutoAddPolicy, SSHClient

from testutil import AWS_REGION, get_free_port, wait_ssh_ready


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

    def run(self, cmd, user, password):
        """
        Run a command on the VM via SSH using the provided credentials.
        """
        if not self.running():
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

    @abc.abstractmethod
    def running(self):
        """
        True if the VM is running.
        """

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.force_stop()


# needed as each distro puts the OVMF.fd in a different location
def find_ovmf():
    for p in [
            "/usr/share/ovmf/OVMF.fd",       # Debian
            "/usr/share/OVMF/OVMF_CODE.fd",  # Fedora
    ]:
        if os.path.exists(p):
            return p
    raise ValueError("cannot find a OVMF bios")


class QEMU(VM):
    MEM = "2000"
    # TODO: support qemu-system-aarch64 too :)
    QEMU = "qemu-system-x86_64"

    def __init__(self, img, snapshot=True, cdrom=None):
        super().__init__()
        self._img = pathlib.Path(img)
        self._qmp_socket = self._img.with_suffix(".qemp-socket")
        self._qemu_p = None
        self._snapshot = snapshot
        self._cdrom = cdrom
        self._ssh_port = None

    def __del__(self):
        self.force_stop()

    # XXX: move args to init() so that __enter__ can use them?
    def start(self, wait_event="ssh", snapshot=True, use_ovmf=False):
        if self.running():
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
            "-qmp", f"unix:{self._qmp_socket},server,nowait",
        ]
        if use_ovmf:
            qemu_cmdline.extend(["-bios", find_ovmf()])
        if self._cdrom:
            qemu_cmdline.extend(["-cdrom", self._cdrom])
        if snapshot:
            qemu_cmdline.append("-snapshot")
        qemu_cmdline.append(self._img)
        self._log(f"vm starting, log available at {log_path}")

        # XXX: use systemd-run to ensure cleanup?
        self._qemu_p = subprocess.Popen(
            qemu_cmdline,
            stdout=sys.stdout,
            stderr=sys.stderr,
        )
        # XXX: also check that qemu is working and did not crash
        ev = wait_event.split(":")
        if ev == ["ssh"]:
            self.wait_ssh_ready()
            self._log(f"vm ready at port {self._ssh_port}")
        elif ev[0] == "qmp":
            qmp_event = ev[1]
            self.wait_qmp_event(qmp_event)
            self._log(f"qmp event {qmp_event}")
        else:
            raise ValueError(f"unsupported wait_event {wait_event}")

    def _wait_qmp_socket(self, timeout_sec):
        for _ in range(timeout_sec):
            if os.path.exists(self._qmp_socket):
                return True
            time.sleep(1)
        raise Exception(f"no {self._qmp_socket} after {timeout_sec} seconds")

    def wait_qmp_event(self, qmp_event):
        # import lazy to avoid requiring it for all operations
        import qmp
        self._wait_qmp_socket(30)
        mon = qmp.QEMUMonitorProtocol(os.fspath(self._qmp_socket))
        mon.connect()
        while True:
            event = mon.pull_event(wait=True)
            self._log(f"DEBUG: got event {event}")
            if event["event"] == qmp_event:
                return

    def force_stop(self):
        if self._qemu_p:
            self._qemu_p.kill()
            self._qemu_p = None
            self._address = None
            self._ssh_port = None

    def running(self):
        return self._qemu_p is not None


class AWS(VM):

    _instance_type = "t3.medium"  # set based on architecture when we add arm tests

    def __init__(self, ami_id):
        super().__init__()
        self._ssh_port = 22
        self._ami_id = ami_id
        self._ec2_instance = None
        self._ec2_security_group = None
        self._ec2_resource = boto3.resource("ec2", region_name=AWS_REGION)

    def start(self):
        if self.running():
            return
        sec_group_ids = []
        if not self._ec2_security_group:
            self._set_ssh_security_group()
        sec_group_ids = [self._ec2_security_group.id]
        try:
            self._log(f"Creating ec2 instance from {self._ami_id}")
            instances = self._ec2_resource.create_instances(
                ImageId=self._ami_id,
                InstanceType=self._instance_type,
                SecurityGroupIds=sec_group_ids,
                MinCount=1, MaxCount=1
            )
            self._ec2_instance = instances[0]
            self._log(f"Waiting for instance {self._ec2_instance.id} to start")
            self._ec2_instance.wait_until_running()
            self._ec2_instance.reload()  # make sure the instance info is up to date
            self._address = self._ec2_instance.public_ip_address
            self._log(f"Instance is running at {self._address}")
            self.wait_ssh_ready()
            self._log("SSH is ready")
        except ClientError as err:
            err_code = err.response["Error"]["Code"]
            err_msg = err.response["Error"]["Message"]
            self._log(f"Couldn't create instance with image {self._ami_id} and type {self._instance_type}.")
            self._log(f"Error {err_code}: {err_msg}")
            raise

    def _set_ssh_security_group(self):
        group_name = f"bootc-image-builder-test-{str(uuid.uuid4())}"
        group_desc = "bootc-image-builder test security group: SSH rule"
        try:
            self._log(f"Creating security group {group_name}")
            self._ec2_security_group = self._ec2_resource.create_security_group(GroupName=group_name,
                                                                                Description=group_desc)
            ip_permissions = [
                {
                    "IpProtocol": "tcp",
                    "FromPort": self._ssh_port,
                    "ToPort": self._ssh_port,
                    "IpRanges": [{"CidrIp": "0.0.0.0/0"}],
                }
            ]
            self._log(f"Authorizing inbound rule for {group_name} ({self._ec2_security_group})")
            self._ec2_security_group.authorize_ingress(IpPermissions=ip_permissions)
            self._log("Security group created")
        except ClientError as err:
            err_code = err.response["Error"]["Code"]
            err_msg = err.response["Error"]["Message"]
            self._log(f"Couldn't create security group {group_name} or authorize inbound rule.")
            self._log(f"Error {err_code}: {err_msg}")
            raise

    def force_stop(self):
        if self._ec2_instance:
            self._log(f"Terminating instance {self._ec2_instance.id}")
            try:
                self._ec2_instance.terminate()
                self._ec2_instance.wait_until_terminated()
                self._ec2_instance = None
                self._address = None
            except ClientError as err:
                err_code = err.response["Error"]["Code"]
                err_msg = err.response["Error"]["Message"]
                self._log(f"Couldn't terminate instance {self._ec2_instance.id}.")
                self._log(f"Error {err_code}: {err_msg}")
        else:
            self._log("No EC2 instance defined. Skipping termination.")

        if self._ec2_security_group:
            self._log(f"Deleting security group {self._ec2_security_group.id}")
            try:
                self._ec2_security_group.delete()
                self._ec2_security_group = None
            except ClientError as err:
                err_code = err.response["Error"]["Code"]
                err_msg = err.response["Error"]["Message"]
                self._log(f"Couldn't delete security group {self._ec2_security_group.id}.")
                self._log(f"Error {err_code}: {err_msg}")
        else:
            self._log("No security group defined. Skipping deletion.")

    def running(self):
        return self._ec2_instance is not None
