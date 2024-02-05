# Developer guide and tooling

The Containerfile contained in this directory is intended to help with working on the multiple components that make up bootc-image-builder. Its purpose is to simplify building the bootc-image-builder container from locally checked out, development versions of the following projects:
1. [osbuild/osbuild](https://github.com/osbuild/osbuild)
2. [osbuild/images](https://github.com/osbuild/images)
3. [osbuild/bootc-image-builder](https://github.com/osbuild/bootc-image-builder) (this repository)

The Containerfile expects two extra build contexts, one for each external component, pointing to the root of the source directory of each project. For example, given the following:
```
$ ls -1 ~/src
images/
osbuild/
```
the container can be built using:
```
$ podman build --file=devel/Containerfile --build-context=osbuild=$HOME/src/osbuild --build-context=images=$HOME/src/images -t bootc-image-builder:devel .
```

**NOTE**: The osbuild RPM build will fail if there are uncommitted changes in the repository.
