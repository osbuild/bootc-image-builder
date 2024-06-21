# Hacking on osbuild/bootc-image-builder

Hacking on `bootc-image-builder` should be fun and is easy.
We have a bunch of unit tests and good integration testing
(including cross-arch image build/testing) based on qemu and
pytest.

## Setup

To work on bootc-image-builder one needs a working Go [compiler/environment?]. See
[go.mod](bib/go.mod). 

To run the testsuite install the test dependencies as outlined in the
[github action](./.github/workflows/tests.yml) under
"Install test dependencies".  Many missing test dependencies will be
auto-detected and the tests skipped. However some (like podman or
qemu) are essential.

## Code layout

The go source code of bib is under `./bib`. It uses the
[images](https://github.com/osbuild/images) library internally to
generate the bootc images. Unit tests (and integration tests where it
makes sense) are expected to be part of a PR but we are happy to
help if those are missing from a PR.

The integration tests are located under `./test` and are written
in pytest.

 
## Build

Build by running:
```
$ cd bib
$ go build ./cmd/bootc-image-builder/
```

## Unit tests

Run the unit tests via:
```
$ cd bib
$ go test ./...
```

## Integration tests

To run the integration tests ensure to have the test dependencies as
outlined above. The integration tests are written in pytest and make
heavy use of the pytest fixtures feature. They are extensive and will
take about 45min to run (dependening on hardware and connection) and
involve building/booting multiple images.

To run them, change into the bootc-image-build root directory and run
```
$ pytest -s -vv
```
for the full output.

Run
```
$ pytest
```
for a more concise output.
