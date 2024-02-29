# Troubleshooting build failures

If your container image fails to build using bootc-image-builder, or if it builds successfully but fails to boot, it's often not clear if the problem lies with bootc-image-builder or the container itself.

To test building an image without bootc-image-builder, try the [bootc-install](devel/bootc-install) script in this repository.

**IMPORTANT**
Before running the script, note that it creates a file called `disk.raw` in the working directory. Make sure this action doesn't overwrite any existing files.

After the disk is created, you can boot test it using qemu. It's best to convert it to a qcow2 image first:
```
qemu-img convert -O qcow disk.raw disk.qcow2
```

You can then follow the [instructions in the README](README.md#running-the-resulting-qcow2-file-on-linux-x86_64) to test that the image boots successfully.
