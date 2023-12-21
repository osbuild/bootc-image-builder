Integration tests for bootc-image-builder
----------------------------------------------

This directory contains integration tests for bootc-image-builder.

They can be run in two ways
1. On the local machine:  
 By just running `sudo pytest -s -v` in the _top level folder_ of the project (where `Containerfile` is)  
 If you have set up `pip` only for your user, you might just want to run the test with elevated privileges  
 `sudo -E $(which pytest) -s -v`
2. Via `tmt` [0] which will spin up a clean VM and run the tests inside: 

	tmt run -vvv

[0] https://github.com/teemtee/tmt

To install `tmt` on fedora at least those packages are needed:

```shell
sudo dnf install tmt tmt+provision-virtual
```
