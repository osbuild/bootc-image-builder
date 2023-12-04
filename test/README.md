Integration tests for bootc-image-builder
----------------------------------------------

This directory contans integration tests for bootc-image-builder.

They can be run in two ways:
1. On the local machine by just running `sudo pytest -s -v`
2. Via `tmt` [0] which will spin up a clean VM and run the tests inside: 

	tmt run -vvv

[0] https://github.com/teemtee/tmt
